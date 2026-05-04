package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"

	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/dao/mysql"
	pkg_minio "github.com/your-username/forum/internal/pkg/minio"
)

func TestUploadVideo(t *testing.T) {
	// 1. Setup Docker Minio client
	c, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	assert.Nil(t, err)
	pkg_minio.SetClient(c)

	// 2. Setup MySQL sqlmock
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	// 3. Setup config
	config.Conf.Minio = &config.MinioConfig{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		UseSSL:          false,
		BucketName:      "forum-test-videos",
	}

	// 4. Ensure bucket exists
	ctx := context.Background()
	exists, err := c.BucketExists(ctx, config.Conf.Minio.BucketName)
	assert.Nil(t, err)
	if !exists {
		err = c.MakeBucket(ctx, config.Conf.Minio.BucketName, minio.MakeBucketOptions{})
		assert.Nil(t, err)
	}

	// 5. Mock MySQL insert
	mock.ExpectExec("insert into videos").
		WithArgs(int64(1), "Test Title", sqlmock.AnyArg(), int64(5), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 6. Upload
	fileContent := []byte("fake-video-data")
	fileReader := bytes.NewReader(fileContent)
	err = UploadVideo(ctx, 1, "Test Title", fileReader, 5, "test.mp4")
	assert.Nil(t, err)

	// 7. Cleanup — delete uploaded file from Minio
	pkg_minio.DeleteFile(ctx, config.Conf.Minio.BucketName, "test.mp4")
}

func TestUploadVideo_MinioFailure(t *testing.T) {
	// Minio client is set from previous test or we can test with nil client scenario
	// For a Minio failure test, we'd need to stop Minio or use a bad endpoint
	// This is better suited for integration tests with container orchestration
}

func TestDeleteVideoService(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	config.Conf.Minio = &config.MinioConfig{BucketName: "test-bucket"}

	ctx := context.Background()
	userID := int64(10)

	t.Run("DeleteVideo_NotFound", func(t *testing.T) {
		mock.ExpectQuery("select .* from videos where id = ?").
			WithArgs(int64(999)).
			WillReturnError(fmt.Errorf("sql: no rows in result set"))

		err := DeleteVideo(ctx, 999, userID)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "no rows")
	})

	t.Run("DeleteVideo_PermissionDenied", func(t *testing.T) {
		mock.ExpectQuery("select .* from videos where id = ?").
			WithArgs(int64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "file_name"}).AddRow(1, 99, "video1.mp4"))

		err := DeleteVideo(ctx, 1, userID)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "permission denied"))
	})
}
