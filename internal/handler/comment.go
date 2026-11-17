package handler

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/your-username/forum/internal/middleware"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/service"
	"net/http"
	"strconv"
	"time"
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := service.SubmitComment(ctx, p); err != nil {
		code := http.StatusServiceUnavailable
		if errors.Is(err, service.ErrInvalidComment) {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, model.CommentSubmission{State: "processing", Comment: p, Fingerprint: p.Fingerprint()})
}

func GetCommentSubmissionHandler(c *gin.Context) {
	uid, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "user not logged in"})
		return
	}
	result, err := service.GetCommentSubmissionForPayload(uid.(int64), c.Param("client_id"), c.Query("fingerprint"))
	if err != nil {
		code := http.StatusServiceUnavailable
		if errors.Is(err, service.ErrInvalidComment) {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func GetCommentListHandler(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid post id"})
		return
	}

	sort := c.DefaultQuery("sort", "hot") // "hot" or "new"

	var uid int64
	if u, ok := c.Get(middleware.ContextUserIDKey); ok {
		uid = u.(int64)
	}

	comments, err := service.GetCommentsWithVotes(postID, sort, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, comments)
}
