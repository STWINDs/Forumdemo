package redis

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/your-username/forum/config"
)

var rdb *redis.Client

func SetRDB(rdb_ *redis.Client) {
	rdb = rdb_
}

func InitResilienceForTest() {
	initResilience()
}

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

var ErrRedisNotReady = fmt.Errorf("redis not connected")

func isReady() error {
	if rdb == nil {
		return ErrRedisNotReady
	}
	return nil
}

func Close() {
	_ = rdb.Close()
}

func DeleteCache(key string) error {
	if err := isReady(); err != nil { return err }
	return rdb.Del(context.Background(), key).Err()
}

// DeleteCacheWithCB 带熔断保护的缓存删除，用于 Canal 消费者
// Redis 不可达时返回错误但不阻塞，依赖 TTL 最终一致性
func DeleteCacheWithCB(key string) error {
	_, err := cb.Execute(func() (interface{}, error) {
		err := rdb.Del(context.Background(), key).Err()
		return nil, err
	})
	return err
}
