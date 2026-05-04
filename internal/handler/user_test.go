package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/your-username/forum/internal/dao/mysql"
)

func TestSignUpHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid_json", func(t *testing.T) {
		r := gin.New()
		r.POST("/signup", SignUpHandler)

		req, _ := http.NewRequest(http.MethodPost, "/signup", bytes.NewBufferString("bad"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("db_error_user_exists", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer mockDB.Close()
		mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

		mock.ExpectQuery("select count\\(id\\) from users where username = ?").
			WithArgs("existing").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		r := gin.New()
		r.POST("/signup", SignUpHandler)

		body, _ := json.Marshal(map[string]string{
			"username": "existing",
			"password": "pass",
			"email":    "a@b.com",
		})
		req, _ := http.NewRequest(http.MethodPost, "/signup", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestLoginHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid_json", func(t *testing.T) {
		r := gin.New()
		r.POST("/login", LoginHandler)

		req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBufferString("bad"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("db_error", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer mockDB.Close()
		mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

		mock.ExpectQuery("select id, username, password from users where username = ?").
			WithArgs("noone").
			WillReturnError(sqlmock.ErrCancelled)

		r := gin.New()
		r.POST("/login", LoginHandler)

		body, _ := json.Marshal(map[string]string{
			"username": "noone",
			"password": "pass",
		})
		req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
