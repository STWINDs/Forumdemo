package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-username/forum/internal/pkg/metrics"
)

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		metrics.HTTPInFlightAdd(1)
		defer metrics.HTTPInFlightAdd(-1)

		c.Next()
		metrics.ObserveHTTP(c.Request.Method, c.FullPath(), c.Writer.Status(), time.Since(started))
	}
}
