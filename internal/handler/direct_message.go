package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/middleware"
	"github.com/your-username/forum/internal/service"
)

type createConversationRequest struct {
	TargetUserID int64 `json:"target_user_id" binding:"required"`
}

type sendDirectMessageRequest struct {
	ClientMessageID string `json:"client_message_id"`
	Content         string `json:"content" binding:"required"`
}

type markConversationReadRequest struct {
	LastReadMessageID int64 `json:"last_read_message_id"`
}

func currentUserID(c *gin.Context) (int64, bool) {
	value, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "user not logged in"})
		return 0, false
	}
	userID, ok := value.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "invalid user context"})
		return 0, false
	}
	return userID, true
}

func parsePositiveID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid " + name})
		return 0, false
	}
	return id, true
}

func FollowUserHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {

		return
	}
	targetID, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	if err := service.FollowUser(c.Request.Context(), userID, targetID); err != nil {
		writeDirectMessageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "followed"})
}

func UnfollowUserHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	targetID, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	if err := service.UnfollowUser(c.Request.Context(), userID, targetID); err != nil {
		writeDirectMessageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "unfollowed"})
}

func GetUserRelationHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	targetID, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	relation, err := service.GetUserRelation(c.Request.Context(), userID, targetID)
	if err != nil {
		writeDirectMessageError(c, err)
		return
	}
	c.JSON(http.StatusOK, relation)
}

func CreateConversationHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var request createConversationRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.TargetUserID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid params"})
		return
	}
	conversation, err := service.GetOrCreateConversation(c.Request.Context(), userID, request.TargetUserID)
	if err != nil {
		writeDirectMessageError(c, err)
		return
	}
	c.JSON(http.StatusOK, conversation)
}

func GetConversationListHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	items, err := service.GetConversationList(c.Request.Context(), userID, limit)
	if err != nil {
		writeDirectMessageError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func GetDirectMessagesHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	conversationID, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	beforeID, _ := strconv.ParseInt(c.DefaultQuery("before_id", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	messages, err := service.GetDirectMessages(c.Request.Context(), conversationID, userID, beforeID, limit)
	if err != nil {
		writeDirectMessageError(c, err)
		return
	}
	c.JSON(http.StatusOK, messages)
}

func SendDirectMessageHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	conversationID, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	var request sendDirectMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid params"})
		return
	}
	result, err := service.SendDirectMessage(c.Request.Context(), conversationID, userID, request.ClientMessageID, request.Content)
	if err != nil {
		writeDirectMessageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

func MarkConversationReadHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	conversationID, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	var request markConversationReadRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid params"})
		return
	}
	if err := service.MarkConversationRead(c.Request.Context(), conversationID, userID, request.LastReadMessageID); err != nil {
		writeDirectMessageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "read"})
}

func GetDirectMessageUnreadCountHandler(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	count, err := service.GetDirectMessageUnreadCount(c.Request.Context(), userID)
	if err != nil {
		writeDirectMessageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}

func writeDirectMessageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDirectMessageEmpty),
		errors.Is(err, service.ErrDirectMessageTooLong),
		errors.Is(err, mysql.ErrCannotMessageSelf):
		c.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
	case errors.Is(err, mysql.ErrorUserNotExist),
		errors.Is(err, mysql.ErrConversationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"msg": err.Error()})
	case errors.Is(err, mysql.ErrConversationForbidden):
		c.JSON(http.StatusForbidden, gin.H{"msg": err.Error()})
	case errors.Is(err, mysql.ErrWaitingForReply),
		errors.Is(err, mysql.ErrStrangerMessageLimit),
		errors.Is(err, mysql.ErrDuplicateClientMessage):
		c.JSON(http.StatusConflict, gin.H{"msg": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
	}
}
