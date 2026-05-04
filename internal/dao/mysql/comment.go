package mysql

import (
	"github.com/your-username/forum/internal/model"
)

func CreateComment(c *model.Comment) (err error) {
	if err = isReady(); err != nil { return }
	sqlStr := `insert into comments(content, author_id, post_id, parent_id) values(?,?,?,?)`
	_, err = db.Exec(sqlStr, c.Content, c.AuthorID, c.PostID, c.ParentID)
	return
}

func GetCommentsByPostID(postID int64) (comments []*model.Comment, err error) {
	if err = isReady(); err != nil { return }
	sqlStr := `select id, content, author_id, post_id, parent_id, status, create_time, update_time
			   from comments where post_id = ? and status = 1 order by create_time desc`
	comments = make([]*model.Comment, 0, 10)
	err = db.Select(&comments, sqlStr, postID)
	return
}
