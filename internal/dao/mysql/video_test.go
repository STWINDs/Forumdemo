package mysql

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/your-username/forum/internal/model"
)

func TestVideoCRUD(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	db = sqlxDB

	video := &model.Video{
		UserID:   1,
		Title:    "test video",
		FileName: "test.mp4",
		Size:     1024,
		URL:      "http://minio/test.mp4",
	}

	// Create
	mock.ExpectExec("insert into videos").
		WithArgs(video.UserID, video.Title, video.FileName, video.Size, video.URL).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = CreateVideo(video)
	assert.Nil(t, err)

	// Get
	now := time.Now()
	mock.ExpectQuery("select .* from videos where id = ?").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "file_name", "size", "url", "create_time"}).
			AddRow(1, 1, "test video", "test.mp4", 1024, "http://minio/test.mp4", now))
	v, err := GetVideoByID(1)
	assert.Nil(t, err)
	assert.Equal(t, video.Title, v.Title)

	// Delete
	mock.ExpectExec("delete from videos").
		WithArgs(1, 1).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = DeleteVideo(1, 1)
	assert.Nil(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}
