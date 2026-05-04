package ratelimit

import (
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
	b := NewTokenBucket(3, 100) // 100 tokens/sec

	// Exhaust tokens
	b.Allow()
	b.Allow()
	b.Allow()
	assert.False(t, b.Allow(), "bucket should be empty")

	// Wait for refill
	time.Sleep(20 * time.Millisecond)
	assert.True(t, b.Allow(), "should refill after wait")
}

func TestAllow_CapacityLimit(t *testing.T) {
	b := NewTokenBucket(2, 1)

	// Wait long enough that refill would exceed capacity
	time.Sleep(10 * time.Millisecond)
	assert.True(t, b.Allow())
	assert.True(t, b.Allow())
	assert.False(t, b.Allow(), "should not exceed capacity")
}
