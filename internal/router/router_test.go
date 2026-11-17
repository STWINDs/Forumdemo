package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/your-username/forum/config"
)

func TestMetricsEndpointBypassesBusinessRateLimit(t *testing.T) {
	r := Setup(gin.TestMode, &config.RateLimitConfig{Enabled: true, Burst: 1, RPS: 0.0001})

	for i := 0; i < 3; i++ {
		request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		require.True(t, strings.Contains(response.Body.String(), "forum_http_in_flight_requests"))
	}
}
