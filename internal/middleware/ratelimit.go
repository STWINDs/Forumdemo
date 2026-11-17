package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/pkg/metrics"
	"github.com/your-username/forum/internal/pkg/ratelimit"
	"net/http"
)

func RateLimitMiddleware(cfg *config.RateLimitConfig) gin.HandlerFunc {
	if cfg == nil || !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}
	bucket := ratelimit.NewTokenBucket(float64(cfg.Burst), cfg.RPS)

	return func(c *gin.Context) {
		// Monitoring must remain reachable when the business traffic bucket is
		// exhausted, otherwise overload hides the evidence needed to diagnose it.
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}
		if !bucket.Allow() {
			metrics.IncRateLimit("rejected")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"msg": "too many requests",
			})
			c.Abort()
			return
		}
		metrics.IncRateLimit("allowed")
		c.Next()
	}
}
