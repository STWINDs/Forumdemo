package mysql

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/your-username/forum/internal/model"
)

func TestCommentReceiptTransaction(t *testing.T) {
	for _, scenario := range []string{"new", "replay", "rollback", "conflict"} {
		t.Run(scenario, func(t *testing.T) {
			raw, mock, err := sqlmock.New()
			assert.NoError(t, err)
			defer raw.Close()
			SetDB(sqlx.NewDb(raw, "mysql"))
			c := &model.Comment{AuthorID: 1, PostID: 2, Content: "hello", ClientCommentID: "request-1"}
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO comment_submissions").WithArgs(int64(1), "request-1").WillReturnResult(sqlmock.NewResult(0, 1))
			rows := sqlmock.NewRows([]string{"comment_id"})
			if scenario == "replay" || scenario == "conflict" {
				rows.AddRow(99)
			} else {
				rows.AddRow(nil)
			}
			mock.ExpectQuery("SELECT comment_id FROM comment_submissions").WithArgs(int64(1), "request-1").WillReturnRows(rows)
			switch scenario {
			case "replay", "conflict":
				content := "hello"
				if scenario == "conflict" {
					content = "different"
				}
				mock.ExpectQuery("SELECT .* FROM comments WHERE id").WithArgs(int64(99)).WillReturnRows(sqlmock.NewRows([]string{"id", "content", "post_id", "parent_id"}).AddRow(99, content, 2, 0))
				if scenario == "conflict" {
					mock.ExpectRollback()
				} else {
					mock.ExpectCommit()
				}
			default:
				exec := mock.ExpectExec("insert into comments").WithArgs("hello", int64(1), int64(2), int64(0))
				if scenario == "rollback" {
					exec.WillReturnError(errors.New("database unavailable"))
					mock.ExpectRollback()
				} else {
					exec.WillReturnResult(sqlmock.NewResult(99, 1))
					mock.ExpectExec("UPDATE comment_submissions").WithArgs(int64(99), int64(1), "request-1").WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectCommit()
				}
			}
			err = CreateComment(c)
			if scenario == "rollback" || scenario == "conflict" {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, 99, c.ID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

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
