package service

import (
	"context"
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/pkg/kafka"
	"testing"
)

func TestCommentSubmissionStatesAreOwnerScoped(t *testing.T) {
	for _, state := range []string{"processing", "published", "unavailable", "database-error"} {
		t.Run(state, func(t *testing.T) {
			raw, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer raw.Close()
			mysql.SetDB(sqlx.NewDb(raw, "mysql"))
			query := mock.ExpectQuery("SELECT c.* FROM comments c JOIN comment_submissions").WithArgs(int64(7), "request-1")
			switch state {
			case "processing":
				query.WillReturnError(sql.ErrNoRows)
			case "database-error":
				query.WillReturnError(sql.ErrConnDone)
			default:
				status := 1
				if state == "unavailable" {
					status = 0
				}
				query.WillReturnRows(sqlmock.NewRows([]string{"id", "author_id", "status"}).AddRow(99, 7, status))
			}
			result, err := GetCommentSubmission(7, "request-1")
			if state == "database-error" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, state, result.State)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCommentSubmissionValidatesBeforeDelivery(t *testing.T) {
	kafka.SetWriter(nil)
	for _, c := range []model.Comment{
		{AuthorID: 1, PostID: 1, Content: "   "},
		{AuthorID: 1, PostID: -1, Content: "hello"},
		{AuthorID: 1, PostID: 1, Content: "hello", ClientCommentID: "bad/request"},
	} {
		require.ErrorIs(t, SubmitComment(context.Background(), &c), ErrInvalidComment)
	}
	c := model.Comment{AuthorID: 1, PostID: 1, Content: "hello", ClientCommentID: "stable-request"}
	require.Error(t, SubmitComment(context.Background(), &c), "unavailable Kafka must never produce a successful receipt")
	require.Equal(t, "stable-request", c.ClientCommentID)
}

func TestFailedPayloadDoesNotHidePublishedComment(t *testing.T) {
	raw, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer raw.Close()
	mysql.SetDB(sqlx.NewDb(raw, "mysql"))
	old := model.Comment{AuthorID: 7, PostID: 9, Content: "original"}
	conflict := old
	conflict.Content = "different"
	expectPublished := func() {
		mock.ExpectQuery("SELECT c.* FROM comments c JOIN comment_submissions").
			WithArgs(int64(7), "request-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "author_id", "post_id", "content", "status"}).AddRow(99, 7, 9, old.Content, 1))
	}
	expectPublished()
	mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(7), "request-1", conflict.Fingerprint()).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	failed, err := GetCommentSubmissionForPayload(7, "request-1", conflict.Fingerprint())
	require.NoError(t, err)
	require.Equal(t, "failed", failed.State)
	require.Nil(t, failed.Comment)
	expectPublished()
	published, err := GetCommentSubmissionForPayload(7, "request-1", old.Fingerprint())
	require.NoError(t, err)
	require.Equal(t, "published", published.State)
	require.NoError(t, mock.ExpectationsWereMet())
}
