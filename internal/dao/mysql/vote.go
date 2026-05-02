package mysql

import (
	"github.com/your-username/forum/internal/model"
)

func CreateVote(v *model.Vote) (err error) {
	// 开启事务或使用唯一索引冲突更新
	sqlStr := `INSERT INTO votes(user_id, post_id, direction)
			   VALUES(?, ?, ?)
			   ON DUPLICATE KEY UPDATE direction = ?`
	_, err = db.Exec(sqlStr, v.UserID, v.PostID, v.Direction, v.Direction)
	return
}
