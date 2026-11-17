package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket 令牌桶
type TokenBucket struct {
	capacity float64   // 桶容量
	rate     float64   // 填充速率（令牌/秒）
	tokens   float64   // 当前令牌数
	lastTick time.Time // 上次填充时间
	now      func() time.Time
	mu       sync.Mutex
}

func NewTokenBucket(capacity, rate float64) *TokenBucket {
	return NewTokenBucketWithClock(capacity, rate, time.Now)
}

// NewTokenBucketWithClock exists so refill behaviour can be verified without
// flaky sleeps. Production callers should use NewTokenBucket.
func NewTokenBucketWithClock(capacity, rate float64, now func() time.Time) *TokenBucket {
	if now == nil {
		now = time.Now
	}
	return &TokenBucket{
		capacity: capacity,
		rate:     rate,
		tokens:   capacity,
		lastTick: now(),
		now:      now,
	}
}

func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	// 计算补充的令牌
	fill := now.Sub(b.lastTick).Seconds() * b.rate
	if fill < 0 {
		fill = 0
	}
	b.tokens += fill
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.lastTick = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}
