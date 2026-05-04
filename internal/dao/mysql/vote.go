package mysql

import (
	"github.com/your-username/forum/internal/model"
)

func CreateVote(v *model.Vote) (err error) {
	if err = isReady(); err != nil { return }
	sqlStr := `INSERT INTO votes(user_id, post_id, direction)
			   VALUES(?, ?, ?)
			   ON DUPLICATE KEY UPDATE direction = ?`
	_, err = db.Exec(sqlStr, v.UserID, v.PostID, v.Direction, v.Direction)
	return
}

func CreateCommentVote(v *model.CommentVote) (err error) {
	if err = isReady(); err != nil { return }
	sqlStr := `INSERT INTO comment_votes(user_id, comment_id, direction)
			   VALUES(?, ?, ?)
			   ON DUPLICATE KEY UPDATE direction = ?`
	_, err = db.Exec(sqlStr, v.UserID, v.CommentID, v.Direction, v.Direction)
	return
}
