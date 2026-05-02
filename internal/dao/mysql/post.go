package mysql

import (
	"github.com/your-username/forum/internal/model"
)

func CreatePost(post *model.Post) (err error) {
	sqlStr := `insert into posts(author_id, title, content, community_id) values(?,?,?,?)`
	_, err = db.Exec(sqlStr, post.AuthorID, post.Title, post.Content, post.CommunityID)
	return
}

func GetPostByID(id int64) (post *model.Post, err error) {
	post = new(model.Post)
	sqlStr := `select id, author_id, title, content, community_id, status, create_time, update_time from posts where id = ?`
	err = db.Get(post, sqlStr, id)
	return
}

func GetPostList(page, size int64) (posts []*model.Post, err error) {
	sqlStr := `select id, author_id, title, content, community_id, status, create_time, update_time from posts order by create_time desc limit ?,?`
	posts = make([]*model.Post, 0, size)
	err = db.Select(&posts, sqlStr, (page-1)*size, size)
	return
}
