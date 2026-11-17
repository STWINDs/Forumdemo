package kafka

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/segmentio/kafka-go"
	"github.com/your-username/forum/internal/model"
	"go.uber.org/zap"
)

type badEvent struct {
	reason  string
	comment *model.Comment
}

func (e *badEvent) Error() string { return e.reason }

var validReceiptID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func validCommentEvent(c model.Comment) bool {
	return c.AuthorID > 0 && c.PostID > 0 && c.ParentID >= 0 && strings.TrimSpace(c.Content) != "" &&
		utf8.RuneCountInString(c.Content) <= 2000 && (c.ClientCommentID == "" || validReceiptID.MatchString(c.ClientCommentID))
}

// The serial consumer calls this repeatedly on the same offset. Remember a
// successful DLQ write while retrying the receipt; restart can still duplicate
// a DLQ record, so operators deduplicate by source coordinate.
func withDeadLetter(
	send func(context.Context, ...kafka.Message) error,
	record func(context.Context, *model.Comment) error,
) func(kafka.Message) error {
	var queued string
	return func(m kafka.Message) error {
		err := processMessage(m)
		var bad *badEvent
		if !errors.As(err, &bad) {
			return err
		}
		coordinate := fmt.Sprintf("%s/%d/%d", m.Topic, m.Partition, m.Offset)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if queued != coordinate {
			headers := append([]kafka.Header(nil), m.Headers...)
			headers = append(headers,
				kafka.Header{Key: "dlq-reason", Value: []byte(bad.reason)},
				kafka.Header{Key: "dlq-source", Value: []byte(coordinate)},
				kafka.Header{Key: "dlq-original-key", Value: m.Key})
			if err := send(ctx, kafka.Message{Key: []byte(coordinate), Value: m.Value, Headers: headers}); err != nil {
				return fmt.Errorf("write dead letter: %w", err)
			}
			queued = coordinate
			zap.L().Error("kafka event quarantined", zap.String("source", coordinate), zap.String("reason", bad.reason))
		}
		if c := bad.comment; c != nil && c.AuthorID > 0 && validReceiptID.MatchString(c.ClientCommentID) {
			if err := record(ctx, c); err != nil {
				return fmt.Errorf("record comment failure: %w", err)
			}
		}
		queued = ""
		return nil
	}
}
