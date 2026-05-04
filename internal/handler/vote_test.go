package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/your-username/forum/internal/middleware"
)

func TestVoteHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid_json", func(t *testing.T) {
		r := gin.New()
		r.POST("/vote", VoteHandler)

		req, _ := http.NewRequest(http.MethodPost, "/vote", bytes.NewBufferString("bad"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := gin.New()
		r.POST("/vote", VoteHandler)

		body, _ := json.Marshal(map[string]string{
			"post_id":   "1",
			"direction": "1",
		})
		req, _ := http.NewRequest(http.MethodPost, "/vote", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid_direction", func(t *testing.T) {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(middleware.ContextUserIDKey, int64(1))
			c.Next()
		})
		r.POST("/vote", VoteHandler)

		body, _ := json.Marshal(map[string]string{
			"post_id":   "1",
			"direction": "5",
		})
		req, _ := http.NewRequest(http.MethodPost, "/vote", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
