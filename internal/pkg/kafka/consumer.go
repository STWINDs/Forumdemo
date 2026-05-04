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
		}
	case "comment_vote":
		var cv model.CommentVote
		if e := json.Unmarshal(m.Value, &cv); e == nil {
			err = retryDBWrite(func() error { return mysql.CreateCommentVote(&cv) })
		}
	case "comment":
		var c model.Comment
		if e := json.Unmarshal(m.Value, &c); e == nil {
			err = retryDBWrite(func() error { return mysql.CreateComment(&c) })
		}
	}
	if err != nil {
		zap.L().Error("consumer db write exhausted retries",
			zap.String("key", string(m.Key)), zap.Error(err))
	}
}

// retryDBWrite 3次退避重试写入 MySQL（应对瞬时 IO 压力）
func retryDBWrite(fn func() error) error {
	var err error
	for i := 0; i < 3; i++ {
		if i > 0 {
			time.Sleep(time.Duration(200<<(i-1)) * time.Millisecond)
		}
		err = fn()
		if err == nil {
			return nil
		}
		zap.L().Warn("consumer db write retry", zap.Int("attempt", i+1), zap.Error(err))
	}
	return err
}
