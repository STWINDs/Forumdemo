package mysql

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/your-username/forum/internal/model"
)

func TestCreateComment(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	db = sqlxDB

	comment := &model.Comment{
		Content:  "test comment",
		AuthorID: 1,
		PostID:   1,
		ParentID: 0,
	}

	mock.ExpectExec("insert into comments").
		WithArgs(comment.Content, comment.AuthorID, comment.PostID, comment.ParentID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = CreateComment(comment)
	assert.Nil(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCommentsByPostID(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	db = sqlxDB

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "content", "author_id", "post_id", "parent_id", "status", "create_time", "update_time"}).
		AddRow(1, "comment 1", 1, 100, 0, 1, now, now).
		AddRow(2, "comment 2", 2, 100, 1, 1, now, now)

	mock.ExpectQuery("select (.+) from comments where post_id = ?").
		WithArgs(100).
		WillReturnRows(rows)

	comments, err := GetCommentsByPostID(100)
	assert.Nil(t, err)
	assert.Len(t, comments, 2)
	assert.Equal(t, "comment 1", comments[0].Content)
	assert.NoError(t, mock.ExpectationsWereMet())
}
