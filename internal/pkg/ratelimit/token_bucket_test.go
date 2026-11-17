package ratelimit

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTokenBucket(t *testing.T) {
	b := NewTokenBucket(10, 2)
	assert.NotNil(t, b)
	assert.Equal(t, float64(10), b.tokens)
}

func TestAllow_WithinCapacity(t *testing.T) {
	b := NewTokenBucket(5, 10)

	for i := 0; i < 5; i++ {
		assert.True(t, b.Allow(), "request %d should be allowed", i)
	}
	assert.False(t, b.Allow(), "6th request should be denied")
}

func TestAllow_Refill(t *testing.T) {
	now := time.Unix(100, 0)
	b := NewTokenBucketWithClock(10, 5, func() time.Time { return now })

	// Exhaust tokens
	for i := 0; i < 10; i++ {
		assert.True(t, b.Allow())
	}
	assert.False(t, b.Allow(), "bucket should be empty")

	now = now.Add(time.Second)
	for i := 0; i < 5; i++ {
		assert.True(t, b.Allow(), "exactly five tokens should refill in one second")
	}
	assert.False(t, b.Allow())
}

func TestAllow_CapacityLimit(t *testing.T) {
	now := time.Unix(100, 0)
	b := NewTokenBucketWithClock(2, 1, func() time.Time { return now })

	now = now.Add(time.Hour)
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.False(t, b.Allow(), "should not exceed capacity")
}

func TestAllow_ConcurrentNeverExceedsBurst(t *testing.T) {
	now := time.Unix(100, 0)
	b := NewTokenBucketWithClock(10, 5, func() time.Time { return now })

	var wg sync.WaitGroup
	allowed := make(chan struct{}, 1000)
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.Allow() {
				allowed <- struct{}{}
			}
		}()
	}
	wg.Wait()
	assert.Len(t, allowed, 10)
}
