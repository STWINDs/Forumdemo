package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/your-username/forum/config"
	"strings"
)

// UniversalClient lets production use Redis Cluster while unit tests keep
// injecting a lightweight standalone/miniredis client.
var rdb redis.UniversalClient

func SetRDB(rdb_ redis.UniversalClient) {
	rdb = rdb_
}

func InitResilienceForTest() {
	initResilience()
}

func Init(cfg *config.RedisConfig) (err error) {
	if cfg == nil {
		return fmt.Errorf("redis config is nil")
	}

	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "cluster" || len(cfg.Addrs) > 0 {
		if cfg.DB != 0 {
			return fmt.Errorf("redis cluster only supports db 0, got %d", cfg.DB)
		}
		if len(cfg.Addrs) == 0 {
			return fmt.Errorf("redis cluster addrs must not be empty")
		}
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    cfg.Addrs,
			Password: cfg.Password,
			PoolSize: cfg.PoolSize,
		})
	} else {
		rdb = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Password: cfg.Password,
			DB:       cfg.DB,
			PoolSize: cfg.PoolSize,
		})
	}

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
	if rdb != nil {
		_ = rdb.Close()
	}
}

func DeleteCache(key string) error {
	if err := isReady(); err != nil {
		return err
	}
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
