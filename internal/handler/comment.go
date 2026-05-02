package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/your-username/forum/internal/middleware"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/service"
	"net/http"
	"strconv"
)

func CreateCommentHandler(c *gin.Context) {
	p := new(model.Comment)
	if err := c.ShouldBindJSON(p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid params"})
		return
	}
	userID, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "user not logged in"})
		return
	}
	p.AuthorID = userID.(int64)
	if err := service.CreateComment(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "success"})
}

func GetCommentListHandler(c *gin.Context) {
	postIDStr := c.Param("post_id")
	postID, err := strconv.ParseInt(postIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid post id"})
		return
	}
	comments, err := service.GetCommentsByPostID(postID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, comments)
}
