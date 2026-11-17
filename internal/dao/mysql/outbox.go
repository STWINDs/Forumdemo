package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	directMessageCacheInvalidationEvent = "direct_message.cache_invalidation"
	outboxPending                       = 0
	outboxProcessing                    = 1
	outboxDone                          = 2
)

type DirectMessageCacheOutbox struct {
	ID             int64   `db:"id"`
	EventType      string  `db:"event_type"`
	ConversationID int64   `db:"conversation_id"`
	MessageID      int64   `db:"message_id"`
	ReceiverID     int64   `db:"receiver_id"`
	UserIDs        []int64 `db:"-"`
	UserIDsJSON    string  `db:"user_ids"`
	Attempts       int     `db:"attempts"`
	LockToken      string  `db:"lock_token"`
}

func enqueueDirectMessageCacheOutbox(ctx context.Context, tx *sqlx.Tx, conversationID, messageID, receiverID int64, userIDs ...int64) (int64, error) {
	encoded, err := json.Marshal(userIDs)
	if err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO direct_message_outbox
		(event_type, conversation_id, message_id, receiver_id, user_ids) VALUES (?, ?, ?, ?, ?)`,
		directMessageCacheInvalidationEvent, conversationID, messageID, receiverID, encoded)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func ClaimDirectMessageCacheOutbox(ctx context.Context, leaseTimeout time.Duration) (*DirectMessageCacheOutbox, error) {
	if err := isReady(); err != nil {
		return nil, err
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var event DirectMessageCacheOutbox
	reclaimBefore := time.Now().Add(-leaseTimeout)
	err = tx.GetContext(ctx, &event, `SELECT id, event_type, conversation_id,
		message_id, receiver_id, user_ids, attempts FROM direct_message_outbox
		WHERE ((status = ? AND available_at <= NOW())
		   OR (status = ? AND locked_at < ?))
		ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`, outboxPending, outboxProcessing, reclaimBefore)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(event.UserIDsJSON), &event.UserIDs); err != nil {
		return nil, err
	}
	event.LockToken = uuid.NewString()
	if _, err := tx.ExecContext(ctx, `UPDATE direct_message_outbox
		SET status = ?, locked_at = NOW(), lock_token = ?, attempts = attempts + 1 WHERE id = ?`,
		outboxProcessing, event.LockToken, event.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &event, nil
}

func CompleteDirectMessageCacheOutbox(ctx context.Context, id int64, lockToken string) error {
	if err := isReady(); err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `UPDATE direct_message_outbox
		SET status = ?, processed_at = NOW(), locked_at = NULL, lock_token = '', last_error = ''
		WHERE id = ? AND status = ? AND lock_token = ?`, outboxDone, id, outboxProcessing, lockToken)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrOutboxLeaseLost
	}
	return nil
}

func RetryDirectMessageCacheOutbox(ctx context.Context, id int64, lockToken string, cause error, retryDelay time.Duration) error {
	if err := isReady(); err != nil {
		return err
	}
	message := "unknown outbox error"
	if cause != nil {
		message = truncateOutboxError(cause.Error(), 512)
	}
	result, err := db.ExecContext(ctx, `UPDATE direct_message_outbox
		SET status = ?, available_at = ?, locked_at = NULL, lock_token = '', last_error = ?
		WHERE id = ? AND status = ? AND lock_token = ?`,
		outboxPending, time.Now().Add(retryDelay), message, id, outboxProcessing, lockToken)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrOutboxLeaseLost
	}
	return nil
}

// CompletePendingDirectMessageCacheOutbox is used only by the synchronous
// post-commit cache path. A pending event has no lease owner yet.
func CompletePendingDirectMessageCacheOutbox(ctx context.Context, id int64) error {
	if err := isReady(); err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `UPDATE direct_message_outbox
		SET status = ?, processed_at = NOW(), last_error = '' WHERE id = ? AND status = ?`, outboxDone, id, outboxPending)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("outbox event is no longer pending")
	}
	return nil
}

func truncateOutboxError(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	for len(value) > maxBytes {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}
