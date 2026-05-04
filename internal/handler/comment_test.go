package handler

import (
	"bytes"
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

func TestCreateCommentHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid_json", func(t *testing.T) {
		r := gin.New()
		r.POST("/comment", CreateCommentHandler)
		req, _ := http.NewRequest(http.MethodPost, "/comment", bytes.NewBufferString("bad"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := gin.New()
		r.POST("/comment", CreateCommentHandler)
		req, _ := http.NewRequest(http.MethodPost, "/comment", bytes.NewBufferString("{}"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestGetCommentListHandler(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	// Setup miniredis for vote counts
	s, _ := miniredis.Run()
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	r := gin.New()
	r.GET("/comments/:id", GetCommentListHandler)

	t.Run("invalid_post_id", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/comments/abc", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("db_error", func(t *testing.T) {
		mock.ExpectQuery("select .* from comments where post_id = ?").
			WithArgs(int64(500)).
			WillReturnError(sqlmock.ErrCancelled)

		// Set a fake user in context
		ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ginCtx.Set(middleware.ContextUserIDKey, int64(1))
		ginCtx.Params = gin.Params{{Key: "id", Value: "500"}}
		ginCtx.Request, _ = http.NewRequest(http.MethodGet, "/comments/500", nil)
		GetCommentListHandler(ginCtx)
		assert.Equal(t, http.StatusInternalServerError, ginCtx.Writer.Status())
	})
}
