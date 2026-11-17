package model

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

type Comment struct {
	ClientCommentID string    `json:"client_comment_id,omitempty" db:"client_comment_id"`
	ID              int64     `json:"id" db:"id"`
	Content         string    `json:"content" db:"content"`
	AuthorID        int64     `json:"author_id" db:"author_id"`
	PostID          int64     `json:"post_id" db:"post_id"`
	ParentID        int64     `json:"parent_id" db:"parent_id"`
	Status          int32     `json:"status" db:"status"`
	CreateTime      time.Time `json:"create_time" db:"create_time"`
	UpdateTime      time.Time `json:"update_time" db:"update_time"`
}

type CommentSubmission struct {
	Fingerprint string   `json:"fingerprint,omitempty"`
	State       string   `json:"state"`
	Comment     *Comment `json:"comment,omitempty"`
}

func (c Comment) Fingerprint() string {
	data, _ := json.Marshal([]any{c.PostID, c.ParentID, c.Content})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
