package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/middleware"
	producer "github.com/your-username/forum/internal/pkg/kafka"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCommentAcceptedWithoutDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: requires Kafka")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	topic := fmt.Sprintf("comment-http-%d", time.Now().UnixNano())
	conn, err := kafka.DialContext(ctx, "tcp", "localhost:29092")
	require.NoError(t, err)
	require.NoError(t, conn.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1}))
	conn.Close()
	writer := &kafka.Writer{Addr: kafka.TCP("localhost:29092"), Topic: topic, RequiredAcks: kafka.RequireAll, BatchSize: 1}
	producer.SetWriter(writer)
	defer func() { writer.Close(); producer.SetWriter(nil) }()
	mysql.SetDB(nil)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(middleware.ContextUserIDKey, int64(1)); c.Next() })
	r.POST("/comment", CreateCommentHandler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/comment", bytes.NewBufferString(`{"content":"accepted before DB persistence","post_id":1,"client_comment_id":"http-receipt"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	var receipt struct {
		State   string `json:"state"`
		Comment struct {
			ID       int64  `json:"id"`
			ClientID string `json:"client_comment_id"`
		} `json:"comment"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &receipt))
	require.Equal(t, "processing", receipt.State)
	require.Zero(t, receipt.Comment.ID)
	require.Equal(t, "http-receipt", receipt.Comment.ClientID)
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{"localhost:29092"}, Topic: topic, Partition: 0})
	defer reader.Close()
	message, err := reader.FetchMessage(ctx)
	require.NoError(t, err)
	require.Contains(t, string(message.Value), "http-receipt")
}
