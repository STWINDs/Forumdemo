package model

import "time"

type Comment struct {
	ID         int64     `json:"id" db:"id"`
	Content    string    `json:"content" db:"content"`
	AuthorID   int64     `json:"author_id" db:"author_id"`
	PostID     int64     `json:"post_id" db:"post_id"`
	ParentID   int64     `json:"parent_id" db:"parent_id"`
	Status     int32     `json:"status" db:"status"`
	CreateTime time.Time `json:"create_time" db:"create_time"`
	UpdateTime time.Time `json:"update_time" db:"update_time"`
}
