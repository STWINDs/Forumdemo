package redis

import (
	"context"
	"fmt"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/model"
	"time"
)

func GetPostDetailWithCache(ctx context.Context, id int64) (post *model.Post, err error) {
	key := fmt.Sprintf("forum:post:%d", id)
	post = new(model.Post)

	err = GetWithResilience(ctx, key, post, time.Hour, func() (interface{}, error) {
		return mysql.GetPostByID(id)
	})
	return
}
