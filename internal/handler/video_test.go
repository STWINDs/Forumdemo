package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestUploadVideoHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("no_file", func(t *testing.T) {
		r := gin.New()
		r.POST("/video/upload", UploadVideoHandler)

		req, _ := http.NewRequest(http.MethodPost, "/video/upload", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unauthorized_no_user", func(t *testing.T) {
		r := gin.New()
		r.POST("/video/upload", UploadVideoHandler)

		body, _ := http.NewRequest(http.MethodPost, "/video/upload", nil) // cannot use simple body for multipart
		req, _ := http.NewRequest(http.MethodPost, "/video/upload", nil)
		_ = body
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code) // no file -> bad request
	})
}

func TestDeleteVideoHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.DELETE("/video/:id", DeleteVideoHandler)

	t.Run("invalid_id", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, "/video/abc", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, "/video/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
