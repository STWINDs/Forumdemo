package middleware

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type contextKey string

const TraceIDKey contextKey = "traceID"
const GinTraceIDKey = "traceID"
const HeaderTraceIDKey = "X-Trace-ID"

func TracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取或生成 Trace ID
		traceID := c.Request.Header.Get(HeaderTraceIDKey)
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// 2. 注入 Gin context
		c.Set(GinTraceIDKey, traceID)

		// 3. 注入标准库 context
		ctx := context.WithValue(c.Request.Context(), TraceIDKey, traceID)
		c.Request = c.Request.WithContext(ctx)

		// 4. 设置响应头
		c.Writer.Header().Set(HeaderTraceIDKey, traceID)

		c.Next()
	}
}
