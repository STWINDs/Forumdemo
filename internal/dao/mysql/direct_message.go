package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/your-username/forum/internal/model"
)

const strangerMessageLimit int64 = 3

func normalizeUserPair(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}

func FollowUser(ctx context.Context, followerID, followeeID int64) error {
	if err := isReady(); err != nil {
		return err
	}
	if followerID == followeeID {
		return ErrCannotMessageSelf
	}

	var exists int
	if err := db.GetContext(ctx, &exists, `SELECT COUNT(*) FROM users WHERE id = ?`, followeeID); err != nil {
		return err
	}
	if exists == 0 {
		return ErrorUserNotExist
	}

	_, err := db.ExecContext(ctx, `INSERT INTO follows(follower_id, followee_id, status)
		VALUES(?, ?, 1)
		ON DUPLICATE KEY UPDATE status = 1, update_time = CURRENT_TIMESTAMP`, followerID, followeeID)
	return err
}

func UnfollowUser(ctx context.Context, followerID, followeeID int64) error {
	if err := isReady(); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `UPDATE follows SET status = 0 WHERE follower_id = ? AND followee_id = ?`, followerID, followeeID)
	return err
}

func GetUserRelation(ctx context.Context, currentUserID, otherUserID int64) (*model.UserRelation, error) {
	if err := isReady(); err != nil {
		return nil, err
	}

	var following, followedBy int64
	err := db.QueryRowxContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM follows WHERE follower_id = ? AND followee_id = ? AND status = 1),
		EXISTS(SELECT 1 FROM follows WHERE follower_id = ? AND followee_id = ? AND status = 1)`,
		currentUserID, otherUserID, otherUserID, currentUserID).Scan(&following, &followedBy)
	if err != nil {
		return nil, err
	}
	return &model.UserRelation{
		Following:  following == 1,
		FollowedBy: followedBy == 1,
		Mutual:     following == 1 && followedBy == 1,
	}, nil
}

func GetOrCreateConversation(ctx context.Context, currentUserID, targetUserID int64) (*model.Conversation, error) {
	if err := isReady(); err != nil {
		return nil, err
	}
	if currentUserID == targetUserID {
		return nil, ErrCannotMessageSelf
	}

	low, high := normalizeUserPair(currentUserID, targetUserID)
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.GetContext(ctx, &exists, `SELECT COUNT(*) FROM users WHERE id IN (?, ?)`, low, high); err != nil {
		return nil, err
	}
	if exists != 2 {
		return nil, ErrorUserNotExist
	}

	if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO conversations(user_low_id, user_high_id) VALUES(?, ?)`, low, high); err != nil {
		return nil, err
	}

	conversation := new(model.Conversation)
	err = tx.GetContext(ctx, conversation, `SELECT id, user_low_id, user_high_id, last_message_id,
		last_message_time, create_time, update_time
		FROM conversations WHERE user_low_id = ? AND user_high_id = ?`, low, high)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return conversation, nil
}

func GetConversationList(ctx context.Context, userID int64, limit int) ([]*model.ConversationSummary, error) {
	if err := isReady(); err != nil {
		return nil, err
	}
	items := make([]*model.ConversationSummary, 0, limit)
	err := db.SelectContext(ctx, &items, `SELECT
		c.id AS conversation_id,
		u.id AS peer_id,
		u.username AS peer_username,
		COALESCE(m.content, '') AS last_message,
		c.last_message_id,
		c.last_message_time AS last_message_at,
		(SELECT COUNT(*) FROM direct_messages unread
		 WHERE unread.conversation_id = c.id AND unread.receiver_id = ? AND unread.status < 3) AS unread_count
		FROM conversations c
		JOIN users u ON u.id = IF(c.user_low_id = ?, c.user_high_id, c.user_low_id)
		LEFT JOIN direct_messages m ON m.id = c.last_message_id
		WHERE c.user_low_id = ? OR c.user_high_id = ?
		ORDER BY c.last_message_time DESC, c.id DESC
		LIMIT ?`, userID, userID, userID, userID, limit)
	return items, err
}

func GetDirectMessages(ctx context.Context, conversationID, userID, beforeID int64, limit int) ([]*model.DirectMessage, error) {
	if err := isReady(); err != nil {
		return nil, err
	}
	if err := ensureConversationMember(ctx, conversationID, userID); err != nil {
		return nil, err
	}

	messages := make([]*model.DirectMessage, 0, limit)
	query := `SELECT id, conversation_id, sender_id, receiver_id, client_message_id,
		content, message_type, status, create_time
		FROM direct_messages WHERE conversation_id = ?`
	args := []interface{}{conversationID}
	if beforeID > 0 {
		query += ` AND id < ?`
		args = append(args, beforeID)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	if err := db.SelectContext(ctx, &messages, query, args...); err != nil {
		return nil, err
	}
	return messages, nil
}

// GetDirectMessageResultByClientID resolves the idempotency key after an
// uncertain commit. The conversation predicate prevents a client ID reused in
// another conversation from returning the wrong message.
func GetDirectMessageResultByClientID(ctx context.Context, conversationID, senderID int64, clientMessageID string) (*model.SendMessageResult, error) {
	if err := isReady(); err != nil {
		return nil, err
	}
	message := new(model.DirectMessage)
	err := db.GetContext(ctx, message, `SELECT id, conversation_id, sender_id, receiver_id,
		client_message_id, content, message_type, status, create_time
		FROM direct_messages WHERE conversation_id = ? AND sender_id = ? AND client_message_id = ?`,
		conversationID, senderID, clientMessageID)
	if err != nil {
		return nil, err
	}

	relation, err := GetUserRelation(ctx, senderID, message.ReceiverID)
	if err != nil {
		return nil, err
	}
	policy := model.SendMessagePolicy{Mutual: relation.Mutual, StrangerRemaining: -1}
	if !relation.Mutual {
		if err := db.GetContext(ctx, &policy.StrangerSentCount, `SELECT COUNT(*) FROM direct_messages
			WHERE conversation_id = ? AND sender_id = ? AND status > 0`, conversationID, senderID); err != nil {
			return nil, err
		}
		policy.StrangerRemaining = strangerMessageLimit - policy.StrangerSentCount
		policy.WaitingForReply = true
	}
	return &model.SendMessageResult{Message: message, Policy: policy}, nil
}

func ensureConversationMember(ctx context.Context, conversationID, userID int64) error {
	var count int
	err := db.GetContext(ctx, &count, `SELECT COUNT(*) FROM conversations
		WHERE id = ? AND (user_low_id = ? OR user_high_id = ?)`, conversationID, userID, userID)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrConversationForbidden
	}
	return nil
}

// CreateDirectMessageWithPolicy demonstrates the important concurrency boundary:
// lock the conversation, evaluate stranger limits, insert the message, and move
// the conversation cursor in one transaction.
func CreateDirectMessageWithPolicy(ctx context.Context, conversationID, senderID int64, clientMessageID, content string) (*model.SendMessageResult, error) {
	if err := isReady(); err != nil {
		return nil, err
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var low, high int64
	err = tx.QueryRowxContext(ctx, `SELECT user_low_id, user_high_id FROM conversations WHERE id = ? FOR UPDATE`, conversationID).Scan(&low, &high)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, err
	}
	if senderID != low && senderID != high {
		return nil, ErrConversationForbidden
	}
	receiverID := low
	if senderID == low {
		receiverID = high
	}

	var mutualCount int64
	err = tx.GetContext(ctx, &mutualCount, `SELECT COUNT(*) FROM follows
		WHERE status = 1 AND ((follower_id = ? AND followee_id = ?) OR (follower_id = ? AND followee_id = ?))`,
		senderID, receiverID, receiverID, senderID)
	if err != nil {
		return nil, err
	}
	mutual := mutualCount == 2

	var sentCount int64
	if !mutual {
		// Check idempotency before stranger-policy rejection. Otherwise a replay
		// of an already committed stranger message would be reported as
		// "waiting for reply" instead of resolving the original message.
		var existingMessageID int64
		err = tx.GetContext(ctx, &existingMessageID, `SELECT id FROM direct_messages
			WHERE conversation_id = ? AND sender_id = ? AND client_message_id = ? LIMIT 1`,
			conversationID, senderID, clientMessageID)
		if err == nil {
			return nil, ErrDuplicateClientMessage
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err := tx.GetContext(ctx, &sentCount, `SELECT COUNT(*) FROM direct_messages
			WHERE conversation_id = ? AND sender_id = ? AND status > 0`, conversationID, senderID); err != nil {
			return nil, err
		}
		if sentCount >= strangerMessageLimit {
			return nil, ErrStrangerMessageLimit
		}

		var lastSenderID int64
		err := tx.GetContext(ctx, &lastSenderID, `SELECT sender_id FROM direct_messages
			WHERE conversation_id = ? AND status > 0 ORDER BY id DESC LIMIT 1`, conversationID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil && lastSenderID == senderID {
			return nil, ErrWaitingForReply
		}
	}

	result, err := tx.ExecContext(ctx, `INSERT INTO direct_messages
		(conversation_id, sender_id, receiver_id, client_message_id, content, message_type, status)
		VALUES(?, ?, ?, ?, ?, 1, 1)`, conversationID, senderID, receiverID, clientMessageID, content)
	if err != nil {
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return nil, ErrDuplicateClientMessage
		}
		return nil, err
	}
	messageID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if _, err := tx.ExecContext(ctx, `UPDATE conversations SET last_message_id = ?, last_message_time = ? WHERE id = ?`, messageID, now, conversationID); err != nil {
		return nil, err
	}
	outboxID, err := enqueueDirectMessageCacheOutbox(ctx, tx, conversationID, messageID, receiverID, senderID, receiverID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if !mutual {
		sentCount++
	}
	remaining := strangerMessageLimit - sentCount
	if mutual {
		remaining = -1 // -1 means unlimited.
	}
	message := &model.DirectMessage{
		ID:              messageID,
		ConversationID:  conversationID,
		SenderID:        senderID,
		ReceiverID:      receiverID,
		ClientMessageID: clientMessageID,
		Content:         content,
		MessageType:     1,
		Status:          1,
		CreateTime:      now,
	}
	return &model.SendMessageResult{
		Message: message,
		Policy: model.SendMessagePolicy{
			Mutual:            mutual,
			StrangerSentCount: sentCount,
			StrangerRemaining: remaining,
			WaitingForReply:   !mutual,
		},
		OutboxID: outboxID,
	}, nil
}

func MarkConversationRead(ctx context.Context, conversationID, userID, lastReadMessageID int64) error {
	if err := isReady(); err != nil {
		return err
	}
	if err := ensureConversationMember(ctx, conversationID, userID); err != nil {
		return err
	}
	query := `UPDATE direct_messages SET status = 3 WHERE conversation_id = ? AND receiver_id = ? AND status < 3`
	args := []interface{}{conversationID, userID}
	if lastReadMessageID > 0 {
		query += ` AND id <= ?`
		args = append(args, lastReadMessageID)
	}
	_, err := db.ExecContext(ctx, query, args...)
	return err
}

func GetDirectMessageUnreadCount(ctx context.Context, userID int64) (int64, error) {
	if err := isReady(); err != nil {
		return 0, err
	}
	var count int64
	err := db.GetContext(ctx, &count, `SELECT COUNT(*) FROM direct_messages WHERE receiver_id = ? AND status < 3`, userID)
	return count, err
}
