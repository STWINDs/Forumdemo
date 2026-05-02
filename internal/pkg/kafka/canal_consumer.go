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

func InitCanalConsumer(cfg *config.KafkaConfig) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{cfg.Address},
		Topic:   "canal-binlog", // Dedicated topic for Canal
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
			processCanalMessage(m)
		}
	}()
}

func processCanalMessage(m kafka.Message) {
	var msg CanalMessage
	if err := json.Unmarshal(m.Value, &msg); err != nil {
		zap.L().Error("unmarshal canal message failed", zap.Error(err))
		return
	}

	for _, row := range msg.Data {
		switch msg.Table {
		case "posts":
			// For posts, we invalidate the cache on any update or delete
			if msg.Type == "UPDATE" || msg.Type == "DELETE" {
				postID := row["id"]
				cacheKey := fmt.Sprintf("forum:post:%v", postID)
				if err := redis.DeleteCache(cacheKey); err != nil {
					zap.L().Error("canal invalidate post cache failed", zap.Error(err), zap.Any("id", postID))
				} else {
					zap.L().Info("canal invalidated post cache", zap.Any("id", postID))
				}
				// 同时失效 L1 缓存
				redis.DelL1(cacheKey)
			}
		}
	}
}
