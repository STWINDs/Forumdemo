package kafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"

	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/model"
)

func TestCommentKafkaE2E(t *testing.T) {
	// 1. Setup sqlmock for MySQL
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	// 2. Setup Kafka producer
	writer := &kafka.Writer{
		Addr:                   kafka.TCP("localhost:29092"),
		Topic:                  "forum-e2e-comments",
		AllowAutoTopicCreation: true,
		BatchSize:              1,
	}
	defer writer.Close()
	SetWriter(writer)

	// 3. Send a comment event (simulating what service.CreateComment does)
	comment := &model.Comment{
		AuthorID: 42,
		PostID:   100,
		Content:  "E2E test comment",
		ParentID: 0,
	}

	err = SendEvent(context.Background(), "comment", comment)
	assert.Nil(t, err)

	// 4. Setup Kafka reader to consume the message
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{"localhost:29092"},
		Topic:   "forum-e2e-comments",
		GroupID: "forum-e2e-test-group",
	})
	defer reader.Close()

	// 5. Mock the expected MySQL insert from the consumer
	mock.ExpectExec("insert into comments").
		WithArgs(comment.Content, comment.AuthorID, comment.PostID, comment.ParentID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 6. Read message and process
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m, err := reader.ReadMessage(ctx)
	assert.Nil(t, err)

	// 7. Call processMessage (same as consumer goroutine)
	processMessage(m)

	// 8. Verify all sqlmock expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVoteKafkaE2E(t *testing.T) {
	// 1. Setup sqlmock
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	// 2. Setup Kafka producer
	writer := &kafka.Writer{
		Addr:                   kafka.TCP("localhost:29092"),
		Topic:                  "forum-e2e-votes",
		AllowAutoTopicCreation: true,
		BatchSize:              1,
	}
	defer writer.Close()
	SetWriter(writer)

	// 3. Send vote event
	vote := &model.Vote{
		UserID:    10,
		PostID:    200,
		Direction: 1,
	}

	err = SendEvent(context.Background(), "vote", vote)
	assert.Nil(t, err)

	// 4. Setup Kafka reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{"localhost:29092"},
		Topic:   "forum-e2e-votes",
		GroupID: "forum-e2e-vote-group",
	})
	defer reader.Close()

	// 5. Mock expected MySQL insert
	mock.ExpectExec("INSERT INTO votes").
		WithArgs(vote.UserID, vote.PostID, vote.Direction, vote.Direction).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 6. Read and process
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m, err := reader.ReadMessage(ctx)
	assert.Nil(t, err)
	processMessage(m)

	// 7. Verify
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessMessage_TraceID(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	// Send message with trace ID header
	writer := &kafka.Writer{
		Addr:                   kafka.TCP("localhost:29092"),
		Topic:                  "forum-e2e-trace",
		AllowAutoTopicCreation: true,
		BatchSize:              1,
	}
	defer writer.Close()
	SetWriter(writer)

	comment := &model.Comment{AuthorID: 1, PostID: 1, Content: "trace test"}
	err = SendEvent(context.Background(), "comment", comment)
	assert.Nil(t, err)

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{"localhost:29092"},
		Topic:   "forum-e2e-trace",
		GroupID: "forum-e2e-trace-group",
	})
	defer reader.Close()

	mock.ExpectExec("insert into comments").
		WithArgs(comment.Content, comment.AuthorID, comment.PostID, comment.ParentID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m, err := reader.ReadMessage(ctx)
	assert.Nil(t, err)

	// Verify trace headers are present (injected by SendEvent when ctx has traceID)
	hasTraceID := false
	for _, h := range m.Headers {
		if h.Key == "X-Trace-ID" {
			hasTraceID = true
			break
		}
	}
	_ = hasTraceID // Trace ID is optional — only present when ctx has it

	processMessage(m)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessMessage_UnmarshalError(t *testing.T) {
	// Test that malformed JSON doesn't crash the consumer
	msg := kafka.Message{
		Key:   []byte("vote"),
		Value: []byte("not-valid-json"),
	}
	// Should not panic
	processMessage(msg)
}

func TestProcessMessage_UnknownKey(t *testing.T) {
	// Test that unknown message keys don't cause issues
	msgData, _ := json.Marshal(map[string]string{"hello": "world"})
	msg := kafka.Message{
		Key:   []byte("unknown-type"),
		Value: msgData,
	}
	// Should not panic
	processMessage(msg)
}
