package kafka

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const (
	voteReconcileInterval = 5 * time.Minute
	voteReconcileDLQFile  = "logs/vote_dlq.json"
)

// DLQEntry 死信队列文件中的一条记录
type DLQEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Timestamp int64  `json:"timestamp"`
}

// StartVoteReconciliationLoop 启动对账定时任务
// 注意：此函数依赖 mysql 和 redis 包的全局变量，调用前确保它们已初始化
func StartVoteReconciliationLoop() {
	go func() {
		ticker := time.NewTicker(voteReconcileInterval)
		defer ticker.Stop()

		for range ticker.C {
			replayDLQFile()
		}
	}()
	zap.L().Info("vote reconciler started", zap.Duration("interval", voteReconcileInterval))
}

// replayDLQFile 重放死信文件中的投票记录
func replayDLQFile() {
	data, err := os.ReadFile(voteReconcileDLQFile)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		zap.L().Error("reconciler cannot read DLQ file", zap.Error(err))
		return
	}

	if len(data) == 0 {
		return
	}

	lines := splitLines(string(data))
	var succeeded int
	var retry []string

	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry DLQEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// 尝试重新投递到 Kafka
		if writer != nil {
			msg := kafka.Message{
				Key:   []byte(entry.Key),
				Value: []byte(entry.Value),
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := writer.WriteMessages(ctx, msg)
			cancel()
			if err != nil {
				retry = append(retry, line)
				zap.L().Warn("reconciler DLQ retry failed",
					zap.String("key", entry.Key), zap.Error(err))
				continue
			}
		}
		succeeded++
	}

	// 重写 DLQ 文件，只保留重试失败的
	if len(retry) > 0 {
		_ = os.WriteFile(voteReconcileDLQFile, []byte(joinLines(retry)), 0644)
	} else {
		_ = os.Truncate(voteReconcileDLQFile, 0)
	}

	if succeeded > 0 {
		zap.L().Info("reconciler replayed DLQ entries",
			zap.Int("succeeded", succeeded),
			zap.Int("retry_remaining", len(retry)))
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for _, l := range lines {
		result += l + "\n"
	}
	return result
}

type VoteRecord struct {
	UserID    int64 `json:"user_id"`
	PostID    int64 `json:"post_id"`
	Direction int8  `json:"direction"`
}

func ParseVoteRecord(value string) (*VoteRecord, error) {
	var v VoteRecord
	if err := json.Unmarshal([]byte(value), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func ParseCommentVoteRecord(value string) (*CommentVoteRecord, error) {
	var v CommentVoteRecord
	if err := json.Unmarshal([]byte(value), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

type CommentVoteRecord struct {
	UserID    int64 `json:"user_id"`
	CommentID int64 `json:"comment_id"`
	Direction int8  `json:"direction"`
}

func itoa(i int64) string {
	return strconv.FormatInt(i, 10)
}
