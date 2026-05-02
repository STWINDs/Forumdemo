package model

import "time"

type Video struct {
	ID         int64     `json:"id" db:"id"`
	UserID     int64     `json:"user_id" db:"user_id"`
	Title      string    `json:"title" db:"title"`
	FileName   string    `json:"file_name" db:"file_name"`
	Size       int64     `json:"size" db:"size"`
	URL        string    `json:"url" db:"url"`
	CreateTime time.Time `json:"create_time" db:"create_time"`
}
