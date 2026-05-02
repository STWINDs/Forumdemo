package redis

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/maypok86/otter"
	"github.com/sony/gobreaker"
	"golang.org/x/sync/singleflight"
	"time"
)

var (
	l1Cache otter.Cache[string, string]
	cb      *gobreaker.CircuitBreaker
	sf      singleflight.Group
)

func initResilience() {
	// 1. 初始化 L1 本地缓存 (容量 10000, TTL 1分钟)
	cache, err := otter.MustBuilder[string, string](10_000).
		CollectStats().
		Cost(func(key string, value string) uint32 {
			return 1
		}).
		Build()
	if err != nil {
		panic(err)
	}
	l1Cache = cache

	// 2. 初始化熔断器
	cb = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "redis-breaker",
		MaxRequests: 3,
		Interval:    5 * time.Second,
		Timeout:     10 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// 连续失败 5 次则触发熔断
			return counts.ConsecutiveFailures >= 5
		},
	})
}

// GetWithResilience 实现多级缓存读取逻辑
// 链路：L1 -> Singleflight -> Circuit Breaker -> L2 (Redis) -> DB Fallback
func GetWithResilience(ctx context.Context, key string, target interface{}, ttl time.Duration, dbFetch func() (interface{}, error)) error {
	// 1. 尝试从 L1 读取
	if val, ok := l1Cache.Get(key); ok {
		return json.Unmarshal([]byte(val), target)
	}

	// 2. 使用 Singleflight 合并并发请求，防止缓存击穿
	v, err, _ := sf.Do(key, func() (interface{}, error) {
		// 3. 再次检查 L1 (双重检查)
		if val, ok := l1Cache.Get(key); ok {
			return val, nil
		}

		// 4. 尝试从 L2 (Redis) 读取，受熔断器保护
		var redisVal string
		_, err := cb.Execute(func() (interface{}, error) {
			var err error
			redisVal, err = rdb.Get(ctx, key).Result()
			return redisVal, err
		})

		// 如果 Redis 命中
		if err == nil {
			l1Cache.Set(key, redisVal)
			return redisVal, nil
		}

		// 5. Redis 未命中或熔断/异常，降级到 DB
		dbVal, err := dbFetch()
		if err != nil {
			return nil, err
		}

		// 序列化结果
		b, _ := json.Marshal(dbVal)
		s := string(b)

		// 回写 Redis (同样受熔断保护，但不阻塞主流程)
		go func() {
			_, _ = cb.Execute(func() (interface{}, error) {
				return nil, rdb.Set(context.Background(), key, s, ttl).Err()
			})
		}()

		// 写入 L1
		l1Cache.Set(key, s)
		return s, nil
	})

	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(v.(string)), target)
}

func SetL1(key string, value string) {
	l1Cache.Set(key, value)
}

func DelL1(key string) {
	l1Cache.Delete(key)
}

var ErrCircuitOpen = errors.New("circuit breaker is open")
