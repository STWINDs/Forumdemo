package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/your-username/forum/internal/middleware"
	"github.com/your-username/forum/internal/service"
	"net/http"
)

type VoteData struct {
	PostID    int64 `json:"post_id,string" binding:"required"`
	Direction int8  `json:"direction,string" binding:"oneof=1 -1"`
}

type CommentVoteData struct {
	CommentID int64 `json:"comment_id,string" binding:"required"`
	Direction int8  `json:"direction,string" binding:"oneof=1 -1"`
}

func VoteHandler(c *gin.Context) {
	var vd VoteData
	if err := c.ShouldBindJSON(&vd); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid params"})
		return
	}

	userID, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "user not logged in"})
		return
	}

	actual, err := service.VoteForPost(userID.(int64), vd.PostID, vd.Direction)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "success", "direction": actual})
}

func CommentVoteHandler(c *gin.Context) {
	var vd CommentVoteData
	if err := c.ShouldBindJSON(&vd); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid params"})
		return
	}

	userID, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "user not logged in"})
		return
	}

	actual, err := service.VoteForComment(userID.(int64), vd.CommentID, vd.Direction)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "success", "direction": actual})
}

func GetPostVotesHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid id"})
		return
	}

	userID, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "user not logged in"})
		return
	}

	info, err := service.GetPostVoteInfo(id, userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}
