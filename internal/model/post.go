package model

import "time"

type Post struct {
	ID           int64     `json:"id" db:"id"`
	AuthorID     int64     `json:"author_id" db:"author_id"`
	Title        string    `json:"title" db:"title"`
	Content      string    `json:"content" db:"content"`
	CommunityID  int64     `json:"community_id" db:"community_id"`
	Status       int32     `json:"status" db:"status"`
	PostType     int8      `json:"post_type" db:"post_type"`         // 1:text 2:link 3:video
	VideoURL     string    `json:"video_url" db:"video_url"`
	ThumbnailURL string    `json:"thumbnail_url" db:"thumbnail_url"`
	CreateTime   time.Time `json:"create_time" db:"create_time"`
	UpdateTime   time.Time `json:"update_time" db:"update_time"`
}
