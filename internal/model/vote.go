package model

import "time"

type Vote struct {
	ID         int64     `json:"id" db:"id"`
	UserID     int64     `json:"user_id" db:"user_id"`
	PostID     int64     `json:"post_id" db:"post_id"`
	Direction  int8      `json:"direction" db:"direction"` // 1:赞成, -1:反对
	CreateTime time.Time `json:"create_time" db:"create_time"`
	UpdateTime time.Time `json:"update_time" db:"update_time"`
}
