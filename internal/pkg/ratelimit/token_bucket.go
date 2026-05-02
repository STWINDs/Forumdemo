package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket 令牌桶
type TokenBucket struct {
	capacity  float64    // 桶容量
	rate      float64    // 填充速率（令牌/秒）
	tokens    float64    // 当前令牌数
	lastTick  time.Time  // 上次填充时间
	mu        sync.Mutex
}

func NewTokenBucket(capacity, rate float64) *TokenBucket {
	return &TokenBucket{
		capacity: capacity,
		rate:     rate,
		tokens:   capacity,
		lastTick: time.Now(),
	}
}

func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	// 计算补充的令牌
	fill := now.Sub(b.lastTick).Seconds() * b.rate
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
