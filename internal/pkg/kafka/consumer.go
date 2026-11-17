package kafka

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/segmentio/kafka-go"
	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/middleware"
	"github.com/your-username/forum/internal/model"
	"go.uber.org/zap"
	"time"
)

func InitConsumer(cfg *config.KafkaConfig) {
	if !IsReady() {
		zap.L().Warn("kafka not ready, consumer disabled")
		return
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{cfg.Address},
		Topic:   cfg.Topic,
		GroupID: "forum-consumer-group",
	})

	go func() {
		defer reader.Close()
		dlq := &kafka.Writer{Addr: kafka.TCP(cfg.Address), Topic: cfg.Topic + "-dlq",
			RequiredAcks: kafka.RequireAll, BatchSize: 1}
		defer dlq.Close()
		_ = consumeReliably(context.Background(), reader, withDeadLetter(dlq.WriteMessages, mysql.RecordCommentSubmissionFailure), time.Second)
	}()
}

type messageReader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
}

// Never fetch the next message until the current one is persisted and committed.
// This also prevents a later offset from acknowledging an earlier failed write.
func consumeReliably(ctx context.Context, reader messageReader, process func(kafka.Message) error, delay time.Duration) error {
	for ctx.Err() == nil {
		m, err := reader.FetchMessage(ctx)
		if err != nil {
			zap.L().Warn("kafka fetch failed; retrying", zap.Error(err))
			if err = waitForRetry(ctx, delay); err != nil {
				return err
			}
			continue
		}
		for attempt := 0; ; attempt++ {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			err = process(m)
			if err == nil {
				break
			}
			zap.L().Error("kafka processing blocked; offset retained", zap.Error(err), zap.Int64("offset", m.Offset), zap.Int("partition", m.Partition), zap.Int("attempt", attempt+1))
			backoff := delay * time.Duration(1<<min(attempt, 5))
			if err = waitForRetry(ctx, backoff); err != nil {
				return err
			}
		}
		for {
			if err = reader.CommitMessages(ctx, m); err == nil {
				break
			}
			zap.L().Warn("kafka commit failed; retrying", zap.Error(err))
			if err = waitForRetry(ctx, delay); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func processMessage(m kafka.Message) error {
	ctx := context.Background()
	for _, h := range m.Headers {
		if h.Key == "X-Trace-ID" {
			ctx = context.WithValue(ctx, middleware.TraceIDKey, string(h.Value))
			break
		}
	}
	_ = ctx

	var err error
	switch string(m.Key) {
	case "vote":
		var v model.Vote
		if e := json.Unmarshal(m.Value, &v); e == nil {
			err = retryDBWrite(func() error { return mysql.CreateVote(&v) })
		} else {
			return &badEvent{reason: "invalid_json"}
		}
	case "comment_vote":
		var cv model.CommentVote
		if e := json.Unmarshal(m.Value, &cv); e == nil {
			err = retryDBWrite(func() error { return mysql.CreateCommentVote(&cv) })
		} else {
			return &badEvent{reason: "invalid_json"}
		}
	case "comment":
		var c model.Comment
		if e := json.Unmarshal(m.Value, &c); e == nil {
			if !validCommentEvent(c) {
				return &badEvent{reason: "invalid_comment_fields", comment: &c}
			}
			if c.ClientCommentID == "" {
				// Legacy queued events get a deterministic identity on replay.
				c.ClientCommentID = fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s/%d/%d", m.Topic, m.Partition, m.Offset))))
			}
			c.Status = 1
			err = retryDBWrite(func() error { return mysql.CreateComment(&c) })
			if errors.Is(err, mysql.ErrCommentIDConflict) {
				return &badEvent{reason: "comment_id_conflict", comment: &c}
			}
		} else {
			return &badEvent{reason: "invalid_json"}
		}
	default:
		return fmt.Errorf("unknown kafka event type: %s", m.Key)
	}
	if err != nil {
		zap.L().Error("consumer db write exhausted retries",
			zap.String("key", string(m.Key)), zap.Error(err))
	}
	return err
}

// retryDBWrite 3次退避重试写入 MySQL（应对瞬时 IO 压力）
func retryDBWrite(fn func() error) error {
	var err error
	for i := 0; i < 3; i++ {
		if i > 0 {
			time.Sleep(time.Duration(200<<(i-1)) * time.Millisecond)
		}
		err = fn()
		if err == nil || errors.Is(err, mysql.ErrCommentIDConflict) {
			return err
		}
		zap.L().Warn("consumer db write retry", zap.Int("attempt", i+1), zap.Error(err))
	}
	return err
}
