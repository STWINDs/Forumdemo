package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/your-username/forum/internal/dao/mysql"
	daoRedis "github.com/your-username/forum/internal/dao/redis"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/pkg/metrics"
	"go.uber.org/zap"
)

const (
	maxDirectMessageLength = 2000
	maxConversationList    = 100
	maxMessagePageSize     = 50
)

var (
	ErrDirectMessageEmpty   = errors.New("message content is required")
	ErrDirectMessageTooLong = errors.New("message content is too long")
)

// DirectMessageEvent is deliberately transport-neutral. A WebSocket hub,
// Kafka producer, or test spy can implement DirectMessagePublisher later.
type DirectMessageEvent struct {
	Type    string               `json:"type"`
	Message *model.DirectMessage `json:"message"`
}

type DirectMessagePublisher interface {
	PublishDirectMessage(ctx context.Context, receiverID int64, event DirectMessageEvent) error
}

type noopDirectMessagePublisher struct{}

func (noopDirectMessagePublisher) PublishDirectMessage(context.Context, int64, DirectMessageEvent) error {
	return nil
}

var directMessagePublisher DirectMessagePublisher = noopDirectMessagePublisher{}

func SetDirectMessagePublisher(publisher DirectMessagePublisher) {
	if publisher == nil {
		directMessagePublisher = noopDirectMessagePublisher{}
		return
	}
	directMessagePublisher = publisher
}

func FollowUser(ctx context.Context, currentUserID, targetUserID int64) error {
	if err := retryTransientMySQL(ctx, func() error {
		return mysql.FollowUser(ctx, currentUserID, targetUserID)
	}); err != nil {
		return err
	}
	logDirectMessageCacheError("invalidate relation after follow",
		daoRedis.InvalidateCachedUserRelation(ctx, currentUserID, targetUserID))
	return nil
}

func UnfollowUser(ctx context.Context, currentUserID, targetUserID int64) error {
	if err := retryTransientMySQL(ctx, func() error {
		return mysql.UnfollowUser(ctx, currentUserID, targetUserID)
	}); err != nil {
		return err
	}
	logDirectMessageCacheError("invalidate relation after unfollow",
		daoRedis.InvalidateCachedUserRelation(ctx, currentUserID, targetUserID))
	return nil
}

func GetUserRelation(ctx context.Context, currentUserID, targetUserID int64) (*model.UserRelation, error) {
	if relation, found, err := daoRedis.GetCachedUserRelation(ctx, currentUserID, targetUserID); err == nil && found {
		return relation, nil
	} else {
		logDirectMessageCacheError("read relation", err)
	}

	relation, err := mysql.GetUserRelation(ctx, currentUserID, targetUserID)
	if err != nil {
		return nil, err
	}
	logDirectMessageCacheError("fill relation",
		daoRedis.SetCachedUserRelation(ctx, currentUserID, targetUserID, relation))
	return relation, nil
}

func GetOrCreateConversation(ctx context.Context, currentUserID, targetUserID int64) (*model.Conversation, error) {
	conversation, err := mysql.GetOrCreateConversation(ctx, currentUserID, targetUserID)
	if err != nil {
		return nil, err
	}
	logDirectMessageCacheError("invalidate lists after conversation creation",
		daoRedis.InvalidateCachedConversationLists(ctx, conversation.ID, currentUserID, targetUserID))
	return conversation, nil
}

func GetConversationList(ctx context.Context, userID int64, limit int) ([]*model.ConversationSummary, error) {
	if limit <= 0 || limit > maxConversationList {
		limit = 30
	}
	if items, found, err := daoRedis.GetCachedConversationList(ctx, userID, limit); err == nil && found {
		return items, nil
	} else {
		logDirectMessageCacheError("read conversation list", err)
	}

	items, err := mysql.GetConversationList(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	logDirectMessageCacheError("fill conversation list",
		daoRedis.SetCachedConversationList(ctx, userID, limit, items))
	return items, nil
}

// GetDirectMessages returns messages in chronological order even though the
// DAO fetches the newest page efficiently with ORDER BY id DESC.
func GetDirectMessages(ctx context.Context, conversationID, userID, beforeID int64, limit int) ([]*model.DirectMessage, error) {
	if limit <= 0 || limit > maxMessagePageSize {
		limit = 30
	}
	messages, err := mysql.GetDirectMessages(ctx, conversationID, userID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func SendDirectMessage(ctx context.Context, conversationID, senderID int64, clientMessageID, content string) (*model.SendMessageResult, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrDirectMessageEmpty
	}
	if len([]rune(content)) > maxDirectMessageLength {
		return nil, ErrDirectMessageTooLong
	}
	if clientMessageID == "" {
		clientMessageID = uuid.NewString()
	}

	result, created, err := createDirectMessageWithRetry(ctx, conversationID, senderID, clientMessageID, content)
	if err != nil {
		return nil, err
	}
	if !created {
		return result, nil
	}
	metrics.IncOutbox("created")

	// MySQL commit is the durability boundary. Cache maintenance is deliberately
	// post-commit and best-effort: a Redis outage must never lose or roll back a
	// follow edge or direct message.
	invalidateErr := daoRedis.InvalidateCachedConversationListsAtVersion(ctx, result.Message.ID, conversationID, senderID, result.Message.ReceiverID)
	unreadErr := daoRedis.IncrementDirectMessageUnreadIfCached(ctx, result.Message.ReceiverID, conversationID)
	logDirectMessageCacheError("invalidate conversation lists after send", invalidateErr)
	logDirectMessageCacheError("increment unread after send", unreadErr)
	if invalidateErr == nil && unreadErr == nil && result.OutboxID > 0 {
		if err := mysql.CompletePendingDirectMessageCacheOutbox(ctx, result.OutboxID); err != nil {
			metrics.IncOutbox("sync_complete_error")
			zap.L().Warn("complete synchronous direct-message outbox failed",
				zap.Int64("outbox_id", result.OutboxID),
				zap.Error(err))
		} else {
			metrics.IncOutbox("sync_completed")
		}
	} else if result.OutboxID > 0 {
		metrics.IncOutbox("left_pending")
	}

	// Persistence is the success boundary. Real-time delivery is best-effort;
	// reconnecting clients recover messages through the history endpoint.
	_ = directMessagePublisher.PublishDirectMessage(ctx, result.Message.ReceiverID, DirectMessageEvent{
		Type:    "message.new",
		Message: result.Message,
	})
	return result, nil
}

var transientRetryDelays = []time.Duration{25 * time.Millisecond, 75 * time.Millisecond}

func retryTransientMySQL(ctx context.Context, operation func() error) error {
	var err error
	for attempt := 0; attempt <= len(transientRetryDelays); attempt++ {
		if err = operation(); err == nil {
			return nil
		}
		if !mysql.IsTransientError(err) || attempt == len(transientRetryDelays) {
			if mysql.IsTransientError(err) {
				metrics.IncDatabase("retry_exhausted")
			}
			return err
		}
		metrics.IncDatabase("retry")
		if err := waitForRetry(ctx, transientRetryDelays[attempt]); err != nil {
			return err
		}
	}
	return err
}

func createDirectMessageWithRetry(ctx context.Context, conversationID, senderID int64, clientMessageID, content string) (*model.SendMessageResult, bool, error) {
	for attempt := 0; attempt <= len(transientRetryDelays); attempt++ {
		result, err := mysql.CreateDirectMessageWithPolicy(ctx, conversationID, senderID, clientMessageID, content)
		if err == nil {
			return result, true, nil
		}
		if errors.Is(err, mysql.ErrDuplicateClientMessage) {
			existing, lookupErr := mysql.GetDirectMessageResultByClientID(ctx, conversationID, senderID, clientMessageID)
			if lookupErr == nil {
				metrics.IncDatabase("idempotency_replay")
				return existing, false, nil
			}
			return nil, false, err
		}
		if !mysql.IsTransientError(err) || attempt == len(transientRetryDelays) {
			if mysql.IsTransientError(err) {
				metrics.IncDatabase("retry_exhausted")
			}
			return nil, false, err
		}
		metrics.IncDatabase("retry")
		if err := waitForRetry(ctx, transientRetryDelays[attempt]); err != nil {
			return nil, false, err
		}
	}
	return nil, false, errors.New("direct message retry exhausted")
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func MarkConversationRead(ctx context.Context, conversationID, userID, lastReadMessageID int64) error {
	if err := mysql.MarkConversationRead(ctx, conversationID, userID, lastReadMessageID); err != nil {
		return err
	}
	logDirectMessageCacheError("invalidate unread after read",
		daoRedis.InvalidateDirectMessageUnreadCache(ctx, userID))
	logDirectMessageCacheError("invalidate conversation list after read",
		daoRedis.InvalidateCachedConversationLists(ctx, conversationID, userID))
	return nil
}

func GetDirectMessageUnreadCount(ctx context.Context, userID int64) (int64, error) {
	if count, found, err := daoRedis.GetCachedDirectMessageUnreadTotal(ctx, userID); err == nil && found {
		return count, nil
	} else {
		logDirectMessageCacheError("read unread total", err)
	}

	count, err := mysql.GetDirectMessageUnreadCount(ctx, userID)
	// 未读消息的总数是一个动态变化的值，缓存可能会过期或被清除，因此即使缓存不存在，也不应返回错误，而是从数据库中重新计算。
	// 此处从数据库中重新计算未读消息总数，并将其写入缓存，以便下次快速访问。
	// 取出后塞入缓存，避免下次再从数据库中查询，减少数据库压力。
	if err != nil {
		return 0, err
	}
	logDirectMessageCacheError("fill unread total",
		daoRedis.SetCachedDirectMessageUnreadTotal(ctx, userID, count))
	return count, nil
}

func logDirectMessageCacheError(operation string, err error) {
	if err == nil || errors.Is(err, daoRedis.ErrRedisNotReady) {
		return
	}
	zap.L().Warn("direct-message cache operation failed",
		zap.String("operation", operation),
		zap.Error(err))
}
