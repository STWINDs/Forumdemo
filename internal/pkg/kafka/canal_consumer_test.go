package kafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"

	dao_redis "github.com/your-username/forum/internal/dao/redis"
)

func TestProcessCanalMessage_InsertPost(t *testing.T) {
	// INSERT events should NOT invalidate cache
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	// Pre-populate cache
	key := "forum:post:1"
	s.Set(key, `{"id":1,"title":"Before"}`)
	dao_redis.SetL1(key, `{"id":1,"title":"Before"}`)

	msg := CanalMessage{
		Type:     "INSERT",
		Table:    "posts",
		Database: "forum",
		Data: []map[string]interface{}{
			{"id": float64(1), "title": "New Post"},
		},
	}
	data, _ := json.Marshal(msg)

	processCanalMessage(kafka.Message{Value: data})

	// Cache should NOT be deleted on INSERT
	val, err := s.Get(key)
	assert.Nil(t, err)
	assert.NotEmpty(t, val)
}

func TestProcessCanalMessage_UpdatePost(t *testing.T) {
	// UPDATE events SHOULD invalidate cache
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	key := "forum:post:1"
	s.Set(key, `{"id":1,"title":"Before"}`)
	dao_redis.SetL1(key, `{"id":1,"title":"Before"}`)

	msg := CanalMessage{
		Type:     "UPDATE",
		Table:    "posts",
		Database: "forum",
		Data: []map[string]interface{}{
			{"id": float64(1), "title": "Updated"},
		},
	}
	data, _ := json.Marshal(msg)

	processCanalMessage(kafka.Message{Value: data})

	// Redis cache should be deleted
	assert.False(t, s.Exists(key))
}

func TestProcessCanalMessage_DeletePost(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	key := "forum:post:1"
	s.Set(key, `{"id":1,"title":"Before"}`)
	dao_redis.SetL1(key, `{"id":1,"title":"Before"}`)

	msg := CanalMessage{
		Type:     "DELETE",
		Table:    "posts",
		Database: "forum",
		Data: []map[string]interface{}{
			{"id": float64(1)},
		},
	}
	data, _ := json.Marshal(msg)

	processCanalMessage(kafka.Message{Value: data})

	// Redis cache should be deleted
	assert.False(t, s.Exists(key))
}

func TestProcessCanalMessage_NonPostTable(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	key := "forum:post:1"
	s.Set(key, `{"id":1,"title":"Before"}`)

	// Event for a non-post table (e.g., "comments")
	msg := CanalMessage{
		Type:     "UPDATE",
		Table:    "comments",
		Database: "forum",
		Data: []map[string]interface{}{
			{"id": float64(1)},
		},
	}
	data, _ := json.Marshal(msg)

	processCanalMessage(kafka.Message{Value: data})

	// Cache should NOT be deleted for non-post tables
	val, err := s.Get(key)
	assert.Nil(t, err)
	assert.NotEmpty(t, val)
}

func TestProcessCanalMessage_InvalidJSON(t *testing.T) {
	// Should return error, not panic
	err := processCanalMessage(kafka.Message{Value: []byte("not-json")})
	assert.NotNil(t, err)
}

// TestProcessCanalMessage_RedisDown_RelyOnTTL verifies graceful degradation
// when Redis is unreachable: the system logs a warning and falls back to TTL-based consistency.
func TestProcessCanalMessage_RedisDown_RelyOnTTL(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	key := "forum:post:1"
	s.Set(key, `{"id":1,"title":"Before"}`)
	dao_redis.SetL1(key, `{"id":1,"title":"Before"}`)

	// Trip the circuit breaker by causing consecutive Redis failures
	// First, close miniredis to simulate Redis going down
	s.Close()

	msg := CanalMessage{
		Type:     "UPDATE",
		Table:    "posts",
		Database: "forum",
		Data:     []map[string]interface{}{{"id": float64(1)}},
	}
	data, _ := json.Marshal(msg)

	// This should NOT panic — TTL-based consistency kicks in
	// L1 deletion still works (in-process), L2 deletion fails gracefully
	err = processCanalMessage(kafka.Message{Value: data})
	assert.Nil(t, err) // L1 deletion succeeds, L2 fails gracefully via CB
}

// TestProcessCanalMessageWithRetry_Success tests that the retry wrapper
// succeeds on first attempt for valid messages.
func TestProcessCanalMessageWithRetry_Success(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	key := "forum:post:1"
	s.Set(key, `{"id":1,"title":"Before"}`)
	dao_redis.SetL1(key, `{"id":1,"title":"Before"}`)

	msg := CanalMessage{
		Type:     "UPDATE",
		Table:    "posts",
		Database: "forum",
		Data:     []map[string]interface{}{{"id": float64(1)}},
	}
	data, _ := json.Marshal(msg)

	err = processCanalMessageWithRetry(kafka.Message{Value: data})
	assert.Nil(t, err)
	assert.False(t, s.Exists(key))
}

// TestProcessCanalMessageWithRetry_Exhausted tests that retry eventually
// returns an error after all attempts fail.
func TestProcessCanalMessageWithRetry_Exhausted(t *testing.T) {
	// Invalid JSON will fail every attempt, testing the retry exhaustion path
	err := processCanalMessageWithRetry(kafka.Message{Value: []byte("{bad json")})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "max retries")
}

// TestSendToDLQ verifies DLQ message is sent when processing fails
func TestSendToDLQ(t *testing.T) {
	// Setup DLQ writer pointing to real Kafka
	dlqWriter = &kafka.Writer{
		Addr:                   kafka.TCP("localhost:29092"),
		Topic:                  "canal-binlog-dlq",
		AllowAutoTopicCreation: true,
		BatchSize:              1,
	}
	defer dlqWriter.Close()

	msg := kafka.Message{
		Key:   []byte("posts"),
		Value: []byte(`{"type":"UPDATE","table":"posts","data":[{"id":1}]}`),
	}

	// This should not panic
	sendToDLQ(msg)

	// Read back from DLQ to verify
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{"localhost:29092"},
		Topic:     "canal-binlog-dlq",
		GroupID:   "dlq-test-group",
		MaxWait:   10 * time.Second,
	})
	defer reader.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dlqMsg, err := reader.ReadMessage(ctx)
	assert.Nil(t, err)
	assert.Equal(t, msg.Value, dlqMsg.Value)
}

func TestProcessCanalMessage_UpdateComments(t *testing.T) {
	// UPDATE events for comments should NOT invalidate post cache
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	key := "forum:post:1"
	s.Set(key, `{"id":1,"title":"Alive"}`)

	msg := CanalMessage{
		Type:     "UPDATE",
		Table:    "comments",
		Database: "forum",
		Data: []map[string]interface{}{
			{"id": float64(1), "content": "new comment"},
		},
	}
	data, _ := json.Marshal(msg)

	processCanalMessage(kafka.Message{Value: data})

	val, err := s.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, `{"id":1,"title":"Alive"}`, val)
}
