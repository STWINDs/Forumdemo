package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/your-username/forum/internal/dao/mysql"
	dao_redis "github.com/your-username/forum/internal/dao/redis"
	"github.com/your-username/forum/internal/middleware"
)

func TestCreatePostHandler(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	s, _ := miniredis.Run()
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	t.Run("invalid_json", func(t *testing.T) {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(middleware.ContextUserIDKey, int64(1))
			c.Next()
		})
		r.POST("/post", CreatePostHandler)
		req, _ := http.NewRequest(http.MethodPost, "/post", httptest.NewRequest(http.MethodPost, "/", nil).Body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec("insert into posts").
			WithArgs(int64(1), "Test Title", "Test Content", int64(1), int8(1), "").
			WillReturnResult(sqlmock.NewResult(1, 1))

		body, _ := json.Marshal(map[string]interface{}{
			"title":        "Test Title",
			"content":      "Test Content",
			"community_id": 1,
			"post_type":    1,
		})

		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(middleware.ContextUserIDKey, int64(1))
			c.Next()
		})
		r.POST("/post", CreatePostHandler)

		req, _ := http.NewRequest(http.MethodPost, "/post", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetPostByIDHandler(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	s, _ := miniredis.Run()
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	r := gin.New()
	r.GET("/post/:id", GetPostByIDHandler)

	t.Run("invalid_id", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/post/abc", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("post_not_found", func(t *testing.T) {
		mock.ExpectQuery("select .* from posts where id = ?").
			WithArgs(int64(999)).
			WillReturnError(context.DeadlineExceeded)

		req, _ := http.NewRequest(http.MethodGet, "/post/999", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestGetPostListHandler(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	r := gin.New()
	r.GET("/posts", GetPostListHandler)

	cols := []string{"id", "author_id", "title", "content", "community_id", "status",
		"post_type", "video_url", "thumbnail_url", "create_time", "update_time"}

	t.Run("default_pagination", func(t *testing.T) {
		rows := sqlmock.NewRows(cols)
		mock.ExpectQuery("select .* from posts order by create_time desc").
			WithArgs(int64(0), int64(10)).
			WillReturnRows(rows)

		req, _ := http.NewRequest(http.MethodGet, "/posts", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
