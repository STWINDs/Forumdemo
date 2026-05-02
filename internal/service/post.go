package service

import (
	"context"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/dao/redis"
	"github.com/your-username/forum/internal/model"
)

func CreatePost(post *model.Post) (err error) {
	return mysql.CreatePost(post)
}

func GetPostByID(ctx context.Context, id int64) (post *model.Post, err error) {
	return redis.GetPostDetailWithCache(ctx, id)
}

func GetPostList(page, size int64) (posts []*model.Post, err error) {
	return mysql.GetPostList(page, size)
}
