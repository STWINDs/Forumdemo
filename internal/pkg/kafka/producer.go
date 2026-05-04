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

func SetWriter(w *kafka.Writer) {
	writer = w
}

func Init(cfg *config.KafkaConfig) {
	if cfg == nil || cfg.Address == "" {
		zap.L().Warn("kafka address not configured, event publishing disabled")
		return
	}
	writer = &kafka.Writer{
		Addr:     kafka.TCP(cfg.Address),
		Topic:    cfg.Topic,
		Balancer: &kafka.LeastBytes{},
	}
}

func IsReady() bool {
	return writer != nil
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

	if writer == nil {
		zap.L().Error("kafka writer is nil, skip sending")
		return nil
	}

	err = writer.WriteMessages(ctx, msg)
	if err != nil {
		zap.L().Error("kafka.WriteMessages failed", zap.Error(err))
	}
	return err
}

func Close() {
	StopAsyncProducer()
	if writer != nil {
		_ = writer.Close()
	}
}
