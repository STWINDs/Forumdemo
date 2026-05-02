package service

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/pkg/minio"
	"io"
	"path"
)

func UploadVideo(ctx context.Context, userID int64, title string, file io.Reader, size int64, fileName string) (err error) {
	// 1. 生成唯一文件名
	ext := path.Ext(fileName)
	newFileName := fmt.Sprintf("%d_%s%s", userID, uuid.New().String(), ext)

	// 2. 上传到 Minio
	contentType := "video/mp4" // 简化处理
	_, err = minio.UploadFile(ctx, config.Conf.Minio.BucketName, newFileName, file, size, contentType)
	if err != nil {
		return err
	}

	// 3. 构造数据库记录
	video := &model.Video{
		UserID:   userID,
		Title:    title,
		FileName: newFileName,
		Size:     size,
		URL:      fmt.Sprintf("/%s/%s", config.Conf.Minio.BucketName, newFileName),
	}

	// 4. 写入 MySQL
	return mysql.CreateVideo(video)
}

func DeleteVideo(ctx context.Context, id, userID int64) error {
	// 1. 查找视频信息
	video, err := mysql.GetVideoByID(id)
	if err != nil {
		return err
	}
	if video.UserID != userID {
		return fmt.Errorf("permission denied")
	}

	// 2. 从 Minio 删除
	err = minio.DeleteFile(ctx, config.Conf.Minio.BucketName, video.FileName)
	if err != nil {
		return err
	}

	// 3. 从 MySQL 删除
	return mysql.DeleteVideo(id, userID)
}
