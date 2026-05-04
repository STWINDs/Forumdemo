package ratelimit

import (
	"testing"
)

func BenchmarkTokenBucket_Allow(b *testing.B) {
	bucket := NewTokenBucket(float64(b.N), float64(b.N))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bucket.Allow()
	}
}

func BenchmarkTokenBucket_Parallel(b *testing.B) {
	bucket := NewTokenBucket(10000, 10000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bucket.Allow()
		}
	})
}
