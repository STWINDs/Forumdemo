package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/dao/mysql"
	daoRedis "github.com/your-username/forum/internal/dao/redis"
	"github.com/your-username/forum/internal/pkg/metrics"
	"go.uber.org/zap"
)

type outboxBatchProcessor func(context.Context, int, time.Duration, time.Duration) (int, error)

// StartDirectMessageOutboxWorker retries cache invalidations recorded in the
// same MySQL transaction as the message.
func StartDirectMessageOutboxWorker(ctx context.Context, cfg *config.DirectMessageOutboxConfig) {
	if cfg == nil {
		cfg = &config.DirectMessageOutboxConfig{
			BatchSize: 100, PollInterval: time.Second, RetryDelay: time.Minute,
			LeaseTimeout: time.Minute, Workers: 1,
		}
	}
	for workerID := 0; workerID < cfg.Workers; workerID++ {
		go runDirectMessageOutboxWorker(ctx, *cfg, processDirectMessageOutboxBatch)
	}
}

func runDirectMessageOutboxWorker(ctx context.Context, cfg config.DirectMessageOutboxConfig, process outboxBatchProcessor) {
	for {
		started := time.Now()
		processed, err := process(ctx, cfg.BatchSize, cfg.LeaseTimeout, cfg.RetryDelay)
		metrics.ObserveOutboxBatch(processed, time.Since(started))
		if err != nil && !errors.Is(err, context.Canceled) {
			metrics.IncOutbox("batch_error")
			zap.L().Warn("process direct-message outbox batch failed", zap.Error(err))
		}
		// A full batch is evidence that more work may already be waiting. Drain
		// it immediately; polling is only for an empty/partial batch or errors.
		if err == nil && processed == cfg.BatchSize {
			continue
		}
		timer := time.NewTimer(cfg.PollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func processDirectMessageOutboxBatch(ctx context.Context, limit int, leaseTimeout, retryDelay time.Duration) (int, error) {
	for processed := 0; processed < limit; processed++ {
		event, err := mysql.ClaimDirectMessageCacheOutbox(ctx, leaseTimeout)
		if errors.Is(err, sql.ErrNoRows) {
			return processed, nil
		}
		if err != nil {
			return processed, err
		}

		cacheErr := daoRedis.InvalidateCachedConversationListsAtVersion(ctx, event.MessageID, event.ConversationID, event.UserIDs...)
		if cacheErr == nil {
			cacheErr = daoRedis.InvalidateDirectMessageUnreadCache(ctx, event.ReceiverID)
		}
		if cacheErr == nil {
			if err := mysql.CompleteDirectMessageCacheOutbox(ctx, event.ID, event.LockToken); err != nil {
				if errors.Is(err, mysql.ErrOutboxLeaseLost) {
					metrics.IncOutbox("lease_lost")
				}
				return processed, err
			}
			metrics.IncOutbox("worker_completed")
			continue
		}
		if err := mysql.RetryDirectMessageCacheOutbox(ctx, event.ID, event.LockToken, cacheErr, retryDelay); err != nil {
			if errors.Is(err, mysql.ErrOutboxLeaseLost) {
				metrics.IncOutbox("lease_lost")
			}
			return processed, err
		}
		metrics.IncOutbox("retry_scheduled")
	}
	return limit, nil
}
