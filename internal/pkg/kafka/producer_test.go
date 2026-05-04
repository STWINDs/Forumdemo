package kafka

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)

func TestSendEvent(t *testing.T) {
	// Connect to real Kafka in Docker (host-accessible port)
	writer := &kafka.Writer{
		Addr:                   kafka.TCP("localhost:29092"),
		Topic:                  "forum-test-events",
		AllowAutoTopicCreation: true,
		BatchSize:              1,
	}
	defer writer.Close()

	// Register writer
	SetWriter(writer)

	err := SendEvent(context.Background(), "test-key", map[string]string{
		"msg": "hello from test",
	})
	assert.Nil(t, err)
}

func TestSendEvent_NilWriter(t *testing.T) {
	SetWriter(nil)

	// Should not panic, just return nil
	// (Previous hack: "return nil if writer is nil" was removed, now it returns after logging)
	// The current code checks writer == nil and returns nil
	err := SendEvent(context.Background(), "key", "value")
	assert.Nil(t, err)
}

func TestSendEvent_MultipleMessages(t *testing.T) {
	writer := &kafka.Writer{
		Addr:                   kafka.TCP("localhost:29092"),
		Topic:                  "forum-test-events",
		AllowAutoTopicCreation: true,
		BatchSize:              1,
	}
	defer writer.Close()

	SetWriter(writer)

	for i := 0; i < 5; i++ {
		err := SendEvent(context.Background(), "batch-key", map[string]int{
			"seq": i,
		})
		assert.Nil(t, err, "message %d should be sent", i)
	}
}
