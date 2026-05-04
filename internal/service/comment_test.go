package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/your-username/forum/internal/dao/mysql"
)

func TestGetCommentsByPostID(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	t.Run("found", func(t *testing.T) {
		postID := int64(1)
		mock.ExpectQuery("select .* from comments where post_id = ?").
			WithArgs(postID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "content"}).AddRow(1, "Comment 1"))

		res, err := GetCommentsByPostID(postID)
		assert.Nil(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "Comment 1", res[0].Content)
	})

	t.Run("not_found", func(t *testing.T) {
		mock.ExpectQuery("select .* from comments where post_id = ?").
			WithArgs(int64(999)).
			WillReturnError(sqlmock.ErrCancelled)

		_, err := GetCommentsByPostID(999)
		assert.NotNil(t, err)
	})

	t.Run("empty_list", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "content", "author_id", "post_id", "parent_id", "status", "create_time", "update_time"})
		mock.ExpectQuery("select .* from comments where post_id = ?").
			WithArgs(int64(500)).
			WillReturnRows(rows)

		res, err := GetCommentsByPostID(500)
		assert.Nil(t, err)
		assert.Len(t, res, 0)
	})
}
