package minio

import (
	"context"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/your-username/forum/config"
	"io"
	"net/url"
	"time"
)

var client *minio.Client

func SetClient(c *minio.Client) {
	client = c
}

func Init(cfg *config.MinioConfig) error {
	var err error
	client, err = minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return err
	}

	// 检查/创建存储桶
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.BucketName)
	if err != nil {
		return err
	}
	if !exists {
		err = client.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func UploadFile(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, contentType string) (string, error) {
	_, err := client.PutObject(ctx, bucketName, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", err
	}
	return objectName, nil
}

func GetPresignedURL(ctx context.Context, bucketName, objectName string, expires time.Duration) (string, error) {
	reqParams := make(url.Values)
	presignedURL, err := client.PresignedGetObject(ctx, bucketName, objectName, expires, reqParams)
	if err != nil {
		return "", err
	}
	return presignedURL.String(), nil
}

func DeleteFile(ctx context.Context, bucketName, objectName string) error {
	return client.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
}
