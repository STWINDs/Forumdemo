package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"

	dao_redis "github.com/your-username/forum/internal/dao/redis"
)

// TestCanalPipelineE2E verifies the full Canal pipeline:
// MySQL binlog → Canal → Kafka → Consumer → Redis cache invalidation
// Uses real Docker Kafka and miniredis for Redis.
func TestCanalPipelineE2E(t *testing.T) {
	// 1. Setup miniredis with pre-populated post cache
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	postID := int64(777)
	cacheKey := "forum:post:777"

	// Pre-populate Redis and L1 cache
	s.Set(cacheKey, `{"id":777,"title":"Original Title"}`)
	dao_redis.SetL1(cacheKey, `{"id":777,"title":"Original Title"}`)

	// 2. Setup Kafka reader with unique group ID to avoid offset conflicts
	groupID := fmt.Sprintf("canal-e2e-update-%d", time.Now().UnixNano())
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{"localhost:29092"},
		Topic:     "canal-binlog",
		GroupID:   groupID,
		StartOffset: kafka.FirstOffset,
		MaxWait:   15 * time.Second,
	})
	defer reader.Close()

	// 3. Send a simulated Canal UPDATE message directly to Kafka
	canalMsg := CanalMessage{
		Type:     "UPDATE",
		Database: "forum",
		Table:    "posts",
		Data: []map[string]interface{}{
			{"id": float64(postID), "title": "Updated via Canal"},
		},
	}
	msgBytes, _ := json.Marshal(canalMsg)

	writer := &kafka.Writer{
		Addr:                   kafka.TCP("localhost:29092"),
		Topic:                  "canal-binlog",
		AllowAutoTopicCreation: false,
		BatchSize:              1,
	}
	defer writer.Close()

	err = writer.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte("posts"),
		Value: msgBytes,
	})
	assert.Nil(t, err)

	// 4. Read the message and process via Canal consumer
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	m, err := reader.ReadMessage(ctx)
	assert.Nil(t, err)

	// 5. Process the message (same as the actual Canal consumer goroutine)
	err = processCanalMessage(m)
	assert.Nil(t, err)

	// 6. Verify Redis cache was invalidated (post updated → cache deleted)
	assert.False(t, s.Exists(cacheKey), "Redis cache should be invalidated after Canal UPDATE event")
}

// TestCanalInsertDoesNotInvalidate verifies that INSERT events don't trigger
// cache invalidation (only UPDATE and DELETE should).
func TestCanalInsertDoesNotInvalidate(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	cacheKey := "forum:post:888"
	s.Set(cacheKey, `{"id":888,"title":"Stays"}`)
	dao_redis.SetL1(cacheKey, `{"id":888,"title":"Stays"}`)

	groupID := fmt.Sprintf("canal-e2e-insert-%d", time.Now().UnixNano())
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{"localhost:29092"},
		Topic:     "canal-binlog",
		GroupID:   groupID,
		StartOffset: kafka.FirstOffset,
		MaxWait:   15 * time.Second,
	})
	defer reader.Close()

	canalMsg := CanalMessage{
		Type:     "INSERT",
		Database: "forum",
		Table:    "posts",
		Data: []map[string]interface{}{
			{"id": float64(888), "title": "New Insert"},
		},
	}
	msgBytes, _ := json.Marshal(canalMsg)

	writer := &kafka.Writer{
		Addr:                   kafka.TCP("localhost:29092"),
		Topic:                  "canal-binlog",
		AllowAutoTopicCreation: false,
		BatchSize:              1,
	}
	defer writer.Close()

	err = writer.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte("posts"),
		Value: msgBytes,
	})
	assert.Nil(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	m, err := reader.ReadMessage(ctx)
	assert.Nil(t, err)

	err = processCanalMessage(m)
	assert.Nil(t, err)

	// Cache should NOT be invalidated for INSERT
	assert.True(t, s.Exists(cacheKey), "Redis cache should NOT be invalidated after Canal INSERT event")
}
