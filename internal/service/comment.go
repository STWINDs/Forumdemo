package service

import (
	"context"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/pkg/kafka"
)

func CreateComment(comment *model.Comment) (err error) {
	// 1. 发送事件到 Kafka
	return kafka.SendEvent(context.Background(), "comment", comment)
}

func GetCommentsByPostID(postID int64) (comments []*model.Comment, err error) {
	return mysql.GetCommentsByPostID(postID)
}
