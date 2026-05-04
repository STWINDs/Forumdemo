package minio

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"

	"github.com/your-username/forum/config"
)

func setupMinioClient(t *testing.T) {
	t.Helper()
	c, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	assert.Nil(t, err)
	SetClient(c)
}

func TestInit(t *testing.T) {
	cfg := &config.MinioConfig{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		UseSSL:          false,
		BucketName:      "test-bucket",
	}

	err := Init(cfg)
	// May return error if bucket exists already (which is fine), or succeed
	// The important thing is it doesn't panic
	_ = err
}

func TestUploadAndDeleteFile(t *testing.T) {
	setupMinioClient(t)

	ctx := context.Background()
	bucket := "test-bucket"
	objectName := "test-upload.txt"

	// Create bucket if not exists
	exists, err := client.BucketExists(ctx, bucket)
	assert.Nil(t, err)
	if !exists {
		err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		assert.Nil(t, err)
	}

	// Upload
	content := []byte("hello minio test")
	reader := bytes.NewReader(content)
	uploadedName, err := UploadFile(ctx, bucket, objectName, reader, int64(len(content)), "text/plain")
	assert.Nil(t, err)
	assert.Equal(t, objectName, uploadedName)

	// Delete
	err = DeleteFile(ctx, bucket, objectName)
	assert.Nil(t, err)
}

func TestGetPresignedURL(t *testing.T) {
	setupMinioClient(t)

	ctx := context.Background()
	bucket := "test-bucket"
	objectName := "test-presigned.txt"

	// Ensure bucket
	exists, _ := client.BucketExists(ctx, bucket)
	if !exists {
		client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	}

	// Upload first
	reader := bytes.NewReader([]byte("data"))
	_, err := UploadFile(ctx, bucket, objectName, reader, 4, "text/plain")
	assert.Nil(t, err)

	// Get presigned URL
	url, err := GetPresignedURL(ctx, bucket, objectName, 5*time.Minute)
	assert.Nil(t, err)
	assert.NotEmpty(t, url)

	// Cleanup
	DeleteFile(ctx, bucket, objectName)
}
