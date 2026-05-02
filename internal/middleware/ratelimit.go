package middleware

import (
	"time"
	"github.com/gin-gonic/gin"
	"github.com/your-username/forum/internal/pkg/ratelimit"
	"net/http"
)

func RateLimitMiddleware(fillInterval time.Duration, cap int64) gin.HandlerFunc {
	// 使用每秒填充速率作为示例
	rate := 1.0 / fillInterval.Seconds()
	bucket := ratelimit.NewTokenBucket(float64(cap), rate)

	return func(c *gin.Context) {
		if !bucket.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"msg": "too many requests",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
