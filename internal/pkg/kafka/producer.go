package kafka

import (
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/middleware"
	"go.uber.org/zap"
)

var writer *kafka.Writer

func Init(cfg *config.KafkaConfig) {
	writer = &kafka.Writer{
		Addr:     kafka.TCP(cfg.Address),
		Topic:    cfg.Topic,
		Balancer: &kafka.LeastBytes{},
	}
}

func SendEvent(ctx context.Context, key string, value interface{}) error {
	b, err := json.Marshal(value)
	if err != nil {
		zap.L().Error("json.Marshal failed", zap.Error(err))
		return err
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: b,
	}

	// 注入 Trace ID 到 Kafka Header
	if traceID, ok := ctx.Value(middleware.TraceIDKey).(string); ok {
		msg.Headers = append(msg.Headers, kafka.Header{
			Key:   string(middleware.HeaderTraceIDKey),
			Value: []byte(traceID),
		})
	}

	err = writer.WriteMessages(ctx, msg)
	if err != nil {
		zap.L().Error("kafka.WriteMessages failed", zap.Error(err))
	}
	return err
}

func Close() {
	if writer != nil {
		_ = writer.Close()
	}
}
