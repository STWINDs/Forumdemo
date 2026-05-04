package mysql

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/your-username/forum/internal/model"
)

func TestCreatePost(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	db = sqlxDB

	post := &model.Post{
		AuthorID:    1,
		Title:       "test title",
		Content:     "test content",
		CommunityID: 1,
	}

	mock.ExpectExec("insert into posts").
		WithArgs(post.AuthorID, post.Title, post.Content, post.CommunityID, post.PostType, post.VideoURL).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = CreatePost(post)
	assert.Nil(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPostByID(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	db = sqlxDB

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "author_id", "title", "content", "community_id", "status", "create_time", "update_time"}).
		AddRow(1, 1, "test title", "test content", 1, 1, now, now)

	mock.ExpectQuery("select (.+) from posts where id = ?").
		WithArgs(1).
		WillReturnRows(rows)

	post, err := GetPostByID(1)
	assert.Nil(t, err)
	assert.NotNil(t, post)
	assert.Equal(t, int64(1), post.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}
