package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/your-username/forum/config"
)

func TestOutboxWorkerImmediatelyContinuesAfterFullBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int64
	secondCall := make(chan struct{})
	processor := func(context.Context, int, time.Duration, time.Duration) (int, error) {
		call := calls.Add(1)
		if call == 1 {
			return 10, nil
		}
		if call == 2 {
			close(secondCall)
			return 0, nil
		}
		return 0, nil
	}
	cfg := config.DirectMessageOutboxConfig{
		BatchSize: 10, PollInterval: time.Hour, RetryDelay: time.Minute,
		LeaseTimeout: time.Minute, Workers: 1,
	}
	go runDirectMessageOutboxWorker(ctx, cfg, processor)

	select {
	case <-secondCall:
		cancel()
	case <-time.After(200 * time.Millisecond):
		t.Fatal("a full batch waited for poll_interval instead of draining immediately")
	}
	require.Equal(t, int64(2), calls.Load())
}
