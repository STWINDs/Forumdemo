package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/your-username/forum/internal/middleware"
	"github.com/your-username/forum/internal/service"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

func UploadVideoHandler(c *gin.Context) {
	// 1. 获取参数
	title := c.PostForm("title")
	file, header, err := c.Request.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid file"})
		return
	}
	defer file.Close()

	// 2. 获取用户 ID
	userID, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "user not logged in"})
		return
	}

	// 3. 调用 service
	err = service.UploadVideo(c.Request.Context(), userID.(int64), title, file, header.Size, header.Filename)
	if err != nil {
		zap.L().Error("service.UploadVideo failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "upload failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": "upload success"})
}

func DeleteVideoHandler(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid id"})
		return
	}

	userID, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "user not logged in"})
		return
	}

	err = service.DeleteVideo(c.Request.Context(), id, userID.(int64))
	if err != nil {
		zap.L().Error("service.DeleteVideo failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": "delete success"})
}
