package service

import (
	"context"
	"strconv"
	"github.com/your-username/forum/internal/dao/redis"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/pkg/kafka"
)

func VoteForPost(userID int64, postID int64, direction int8) error {
	// 1. Redis 中更新分数和投票纪录
	err := redis.VoteForPost(strconv.FormatInt(userID, 10), strconv.FormatInt(postID, 10), float64(direction))
	if err != nil {
		return err
	}

	// 2. 发送事件到 Kafka，由消费者异步持久化到 MySQL
	vote := &model.Vote{
		UserID:    userID,
		PostID:    postID,
		Direction: direction,
	}
	return kafka.SendEvent(context.Background(), "vote", vote)
}
