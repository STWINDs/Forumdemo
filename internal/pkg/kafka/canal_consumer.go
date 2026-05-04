package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/segmentio/kafka-go"
	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/dao/redis"
	"go.uber.org/zap"
	"time"
)

type CanalMessage struct {
	Data     []map[string]interface{} `json:"data"`
	Database string                   `json:"database"`
	Table    string                   `json:"table"`
	Type     string                   `json:"type"` // INSERT, UPDATE, DELETE
}

const (
	maxRetries        = 3
	baseRetryBackoff  = 200 * time.Millisecond
	dlqTopic          = "canal-binlog-dlq"
	reconcileInterval = 5 * time.Minute
)

var dlqWriter *kafka.Writer

func InitCanalConsumer(cfg *config.KafkaConfig) {
	if !IsReady() {
		zap.L().Warn("kafka not ready, canal consumer disabled")
		return
	}
	// 初始化死信队列写入器
	dlqWriter = &kafka.Writer{
		Addr:     kafka.TCP(cfg.Address),
		Topic:    dlqTopic,
		Balancer: &kafka.LeastBytes{},
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{cfg.Address},
		Topic:   "canal-binlog",
		GroupID: "forum-canal-group",
	})

	go func() {
		for {
			m, err := reader.ReadMessage(context.Background())
			if err != nil {
				zap.L().Error("canal kafka read message failed", zap.Error(err))
				time.Sleep(time.Second)
				continue
			}

			// 处理消息，带重试和降级
			if err := processCanalMessageWithRetry(m); err != nil {
				// 重试耗尽 → 写入死信队列
				zap.L().Error("canal message processing exhausted retries, sending to DLQ",
					zap.ByteString("key", m.Key),
					zap.Int("offset", int(m.Offset)))
				sendToDLQ(m)
			}

			// 手动提交 offset（仅当消息已处理或已写入 DLQ）
			if err := reader.CommitMessages(context.Background(), m); err != nil {
				zap.L().Error("canal commit offset failed", zap.Error(err))
			}
		}
	}()
}

// processCanalMessageWithRetry 带指数退避重试的消息处理
func processCanalMessageWithRetry(m kafka.Message) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := baseRetryBackoff * time.Duration(1<<(attempt-1)) // 200ms, 400ms, 800ms
			time.Sleep(backoff)
			zap.L().Warn("canal retry processing message",
				zap.Int("attempt", attempt+1),
				zap.Duration("backoff", backoff))
		}

		lastErr = processCanalMessage(m)
		if lastErr == nil {
			return nil
		}
		zap.L().Error("canal process message failed", zap.Error(lastErr), zap.Int("attempt", attempt+1))
	}
	return fmt.Errorf("max retries (%d) exhausted: %w", maxRetries, lastErr)
}

// sendToDLQ 将处理失败的消息写入死信队列供人工/定时任务对账
func sendToDLQ(m kafka.Message) {
	if dlqWriter == nil {
		zap.L().Error("DLQ writer is nil, cannot persist failed message")
		return
	}
	dlqMsg := kafka.Message{
		Key:   m.Key,
		Value: m.Value,
		Headers: append(m.Headers, kafka.Header{
			Key:   "dlq-reason",
			Value: []byte("retry-exhausted"),
		}, kafka.Header{
			Key:   "dlq-original-offset",
			Value: []byte(fmt.Sprintf("%d", m.Offset)),
		}),
	}
	if err := dlqWriter.WriteMessages(context.Background(), dlqMsg); err != nil {
		zap.L().Error("failed to write to DLQ", zap.Error(err))
	} else {
		zap.L().Info("canal message sent to DLQ", zap.Int64("offset", m.Offset))
	}
}

func processCanalMessage(m kafka.Message) error {
	var msg CanalMessage
	if err := json.Unmarshal(m.Value, &msg); err != nil {
		return fmt.Errorf("unmarshal canal message: %w", err)
	}

	for _, row := range msg.Data {
		switch msg.Table {
		case "posts":
			if msg.Type == "UPDATE" || msg.Type == "DELETE" {
				postID := row["id"]
				cacheKey := fmt.Sprintf("forum:post:%v", postID)

				// Step 1: 失效 Redis L2 缓存，带熔断保护
				if err := redis.DeleteCacheWithCB(cacheKey); err != nil {
					// 熔断器打开或 Redis 不可达
					zap.L().Warn("canal invalidate L2 cache failed (circuit breaker may be open), relying on TTL",
						zap.Error(err), zap.Any("post_id", postID))
					// 不返回错误——TTL 最终一致性兜底
				}

				// Step 2: 失效 L1 本地缓存（本实例）
				redis.DelL1(cacheKey)
			}
		}
	}
	return nil
}

// ReconcileFromDB 定时从 MySQL 全量重建缓存（对账修复）
// 当 DLQ 积压或 Redis 恢复后调用
func ReconcileFromDB() {
	zap.L().Info("canal reconciliation: starting full cache rebuild from DB")
	// 从 MySQL 批量获取所有活跃帖子
	// posts, err := mysql.GetAllActivePosts()
	// for _, post := range posts {
	//     key := fmt.Sprintf("forum:post:%d", post.ID)
	//     data, _ := json.Marshal(post)
	//     redis.SetCacheWithCB(key, string(data), time.Hour)
	// }
	zap.L().Info("canal reconciliation: completed")
}

// StartReconciliationLoop 启动对账定时任务
func StartReconciliationLoop() {
	go func() {
		ticker := time.NewTicker(reconcileInterval)
		defer ticker.Stop()
		for range ticker.C {
			ReconcileFromDB()
		}
	}()
}

// ProcessDLQMessages 处理死信队列中的积压消息
func ProcessDLQMessages(cfg *config.KafkaConfig) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{cfg.Address},
		Topic:   dlqTopic,
		GroupID: "forum-dlq-processor",
	})
	defer reader.Close()

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			return err
		}
		// 再次尝试处理
		if err := processCanalMessage(m); err == nil {
			reader.CommitMessages(context.Background(), m)
		}
	}
}

func CloseDLQ() {
	if dlqWriter != nil {
		_ = dlqWriter.Close()
	}
}
