package model

import "time"

type Post struct {
	ID          int64     `json:"id" db:"id"`
	AuthorID    int64     `json:"author_id" db:"author_id"`
	Title       string    `json:"title" db:"title"`
	Content     string    `json:"content" db:"content"`
	CommunityID int64     `json:"community_id" db:"community_id"`
	Status      int32     `json:"status" db:"status"`
	CreateTime  time.Time `json:"create_time" db:"create_time"`
	UpdateTime  time.Time `json:"update_time" db:"update_time"`
}
