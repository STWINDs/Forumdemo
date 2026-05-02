package redis

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/your-username/forum/config"
)

var rdb *redis.Client

func Init(cfg *config.RedisConfig) (err error) {
	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	_, err = rdb.Ping(context.Background()).Result()
	if err == nil {
		initResilience()
	}
	return err
}

func Close() {
	_ = rdb.Close()
}

func DeleteCache(key string) error {
	return rdb.Del(context.Background(), key).Err()
}
