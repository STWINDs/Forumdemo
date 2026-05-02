package kafka

import (
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/middleware"
	"github.com/your-username/forum/internal/model"
	"go.uber.org/zap"
	"time"
)

func InitConsumer(cfg *config.KafkaConfig) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{cfg.Address},
		Topic:   cfg.Topic,
		GroupID: "forum-consumer-group",
	})

	go func() {
		for {
			m, err := reader.ReadMessage(context.Background())
			if err != nil {
				zap.L().Error("kafka read message failed", zap.Error(err))
				time.Sleep(time.Second)
				continue
			}
			processMessage(m)
		}
	}()
}

func processMessage(m kafka.Message) {
	// 提取 Trace ID 注入 context 用于日志记录
	ctx := context.Background()
	for _, h := range m.Headers {
		if h.Key == "X-Trace-ID" {
			ctx = context.WithValue(ctx, middleware.TraceIDKey, string(h.Value))
			break
		}
	}

	switch string(m.Key) {
	case "vote":
		var v model.Vote
		if err := json.Unmarshal(m.Value, &v); err == nil {
			_ = mysql.CreateVote(&v)
		}
	case "comment":
		var c model.Comment
		if err := json.Unmarshal(m.Value, &c); err == nil {
			_ = mysql.CreateComment(&c)
		}
	}
}
