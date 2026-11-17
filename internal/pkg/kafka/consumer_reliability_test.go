package kafka

import (
	"context"
	"errors"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type controlledReader struct {
	fetch  func(context.Context) (kafka.Message, error)
	commit func(context.Context, ...kafka.Message) error
}

func (r controlledReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	return r.fetch(ctx)
}
func (r controlledReader) CommitMessages(ctx context.Context, m ...kafka.Message) error {
	return r.commit(ctx, m...)
}

func TestConsumerRetainsMessageUntilRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	fetches, attempts, commits := 0, 0, 0
	r := controlledReader{
		fetch: func(context.Context) (kafka.Message, error) {
			fetches++
			require.Zero(t, commits)
			return kafka.Message{Offset: 42}, nil
		},
		commit: func(_ context.Context, m ...kafka.Message) error {
			require.Equal(t, 3, attempts, "must not acknowledge failed DB writes")
			require.EqualValues(t, 42, m[0].Offset)
			commits++
			if commits == 1 {
				return errors.New("broker temporarily unavailable")
			}
			cancel()
			return nil
		},
	}
	err := consumeReliably(ctx, r, func(m kafka.Message) error {
		attempts++
		require.EqualValues(t, 42, m.Offset)
		if attempts < 3 {
			return errors.New("database offline")
		}
		return nil
	}, time.Millisecond)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, fetches, "no later offset may overtake the failed event")
	require.Equal(t, 2, commits)
}

func TestConsumerShutdownLeavesFailedMessageUncommitted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	commits := 0
	r := controlledReader{fetch: func(context.Context) (kafka.Message, error) { return kafka.Message{Offset: 42}, nil }, commit: func(context.Context, ...kafka.Message) error { commits++; return nil }}
	err := consumeReliably(ctx, r, func(kafka.Message) error { cancel(); return errors.New("database offline") }, time.Millisecond)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, commits)
}
