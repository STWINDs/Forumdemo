package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/your-username/forum/internal/pkg/jwt"
)

func TestJWTAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token, _ := jwt.GenToken(1, "testuser")

	tests := []struct {
		name       string
		setHeader  bool
		headerVal  string
		wantStatus int
	}{
		{"empty_header", false, "", http.StatusUnauthorized},
		{"bad_format_no_bearer", true, "bad-token", http.StatusUnauthorized},
		{"bad_format_wrong_prefix", true, "Basic xxx", http.StatusUnauthorized},
		{"invalid_token", true, "Bearer invalid.token.here", http.StatusUnauthorized},
		{"valid_token", true, "Bearer " + token, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(JWTAuthMiddleware())
			r.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req, _ := http.NewRequest(http.MethodGet, "/test", nil)
			if tt.setHeader {
				req.Header.Set("Authorization", tt.headerVal)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestJWTAuthMiddleware_SetsUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token, _ := jwt.GenToken(99, "bob")

	r := gin.New()
	r.Use(JWTAuthMiddleware())
	r.GET("/me", func(c *gin.Context) {
		userID, ok := c.Get(ContextUserIDKey)
		assert.True(t, ok)
		assert.Equal(t, int64(99), userID)
		c.JSON(http.StatusOK, gin.H{"id": userID})
	})

	req, _ := http.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var res map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &res)
	assert.Equal(t, float64(99), res["id"])
}

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// High rate — all requests allowed
	r := gin.New()
	r.Use(RateLimitMiddleware(10*time.Millisecond, 100))
	r.GET("/api", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest(http.MethodGet, "/api", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should pass", i)
	}

	// Tight limit — some requests should fail
	r2 := gin.New()
	r2.Use(RateLimitMiddleware(1*time.Second, 1))
	r2.GET("/limited", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// First request consumes the only token
	req1, _ := http.NewRequest(http.MethodGet, "/limited", nil)
	w1 := httptest.NewRecorder()
	r2.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request should be rate-limited
	req2, _ := http.NewRequest(http.MethodGet, "/limited", nil)
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
}

func TestTracingMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("generates_trace_id", func(t *testing.T) {
		r := gin.New()
		r.Use(TracingMiddleware())
		r.GET("/trace", func(c *gin.Context) {
			traceID, ok := c.Get(GinTraceIDKey)
			assert.True(t, ok)
			assert.NotEmpty(t, traceID)
			c.JSON(http.StatusOK, gin.H{"trace": traceID})
		})

		req, _ := http.NewRequest(http.MethodGet, "/trace", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Response header should contain trace ID
		assert.NotEmpty(t, w.Header().Get(HeaderTraceIDKey))
	})

	t.Run("propagates_existing_trace_id", func(t *testing.T) {
		r := gin.New()
		r.Use(TracingMiddleware())
		r.GET("/trace", func(c *gin.Context) {
			traceID, _ := c.Get(GinTraceIDKey)
			c.JSON(http.StatusOK, gin.H{"trace": traceID})
		})

		req, _ := http.NewRequest(http.MethodGet, "/trace", nil)
		req.Header.Set(HeaderTraceIDKey, "my-custom-trace-id")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, "my-custom-trace-id", w.Header().Get(HeaderTraceIDKey))

		var res map[string]string
		json.Unmarshal(w.Body.Bytes(), &res)
		assert.Equal(t, "my-custom-trace-id", res["trace"])
	})

	t.Run("context_propagates_to_request_context", func(t *testing.T) {
		r := gin.New()
		r.Use(TracingMiddleware())
		r.GET("/ctx", func(c *gin.Context) {
			traceID := c.Request.Context().Value(TraceIDKey)
			assert.NotNil(t, traceID)
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		req, _ := http.NewRequest(http.MethodGet, "/ctx", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
