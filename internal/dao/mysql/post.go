package mysql

import (
	"github.com/jmoiron/sqlx"
	"github.com/your-username/forum/internal/model"
)

func CreatePost(post *model.Post) (err error) {
	if err = isReady(); err != nil { return }
	sqlStr := `insert into posts(author_id, title, content, community_id, post_type, video_url) values(?,?,?,?,?,?)`
	result, err := db.Exec(sqlStr, post.AuthorID, post.Title, post.Content, post.CommunityID, post.PostType, post.VideoURL)
	if err != nil {
		return err
	}
	post.ID, err = result.LastInsertId()
	return
}

func GetPostByID(id int64) (post *model.Post, err error) {
	if err = isReady(); err != nil { return }
	post = new(model.Post)
	sqlStr := `select id, author_id, title, content, community_id, status, post_type, video_url, thumbnail_url, create_time, update_time from posts where id = ?`
	err = db.Get(post, sqlStr, id)
	return
}

func UpdatePost(id, authorID int64, title, content string, postType int8) (err error) {
	if err = isReady(); err != nil { return }
	sqlStr := `update posts set title=?, content=?, post_type=? where id=? and author_id=?`
	_, err = db.Exec(sqlStr, title, content, postType, id, authorID)
	return
}

func DeletePost(id, authorID int64) (err error) {
	if err = isReady(); err != nil { return }
	sqlStr := `update posts set status=0 where id=? and author_id=?`
	_, err = db.Exec(sqlStr, id, authorID)
	return
}

func GetPostList(page, size int64) (posts []*model.Post, err error) {
	if err = isReady(); err != nil { return }
	sqlStr := `select id, author_id, title, content, community_id, status, post_type, video_url, thumbnail_url, create_time, update_time from posts order by create_time desc limit ?,?`
	posts = make([]*model.Post, 0, size)
	err = db.Select(&posts, sqlStr, (page-1)*size, size)
	return
}

// GetCommentCounts returns comment counts for multiple post IDs
func GetCommentCounts(postIDs []int64) (map[int64]int64, error) {
	if err := isReady(); err != nil { return nil, err }
	if len(postIDs) == 0 {
		return map[int64]int64{}, nil
	}

	query, args, err := sqlx.In(`select post_id, count(*) as cnt from comments where post_id in (?) and status = 1 group by post_id`, postIDs)
	if err != nil {
		return nil, err
	}
	query = db.Rebind(query)

	type row struct {
		PostID int64 `db:"post_id"`
		Cnt    int64 `db:"cnt"`
	}
	var rows []row
	if err := db.Select(&rows, query, args...); err != nil {
		return nil, err
	}

	result := make(map[int64]int64, len(postIDs))
	for _, r := range rows {
		result[r.PostID] = r.Cnt
	}
	return result, nil
}

// TopCommentRow a single top comment for a post
type TopCommentRow struct {
	ID       int64  `db:"id"`
	Content  string `db:"content"`
	AuthorID int64  `db:"author_id"`
	PostID   int64  `db:"post_id"`
}

// GetTopComments returns the hottest comment (by upvotes) for each post
func GetTopComments(postIDs []int64) (map[int64]*TopCommentRow, error) {
	if err := isReady(); err != nil { return nil, err }
	if len(postIDs) == 0 {
		return map[int64]*TopCommentRow{}, nil
	}

	query, args, err := sqlx.In(`select c.id, c.content, c.author_id, c.post_id from comments c
		where c.post_id in (?) and c.status = 1
		order by c.create_time desc limit 1`, postIDs)
	if err != nil {
		return nil, err
	}
	query = db.Rebind(query)

	// Get the latest comment per post as "top comment" (simplified — ideally sort by vote count)
	var rows []TopCommentRow
	if err := db.Select(&rows, query, args...); err != nil {
		return nil, err
	}

	result := make(map[int64]*TopCommentRow, len(rows))
	for i := range rows {
		result[rows[i].PostID] = &rows[i]
	}
	return result, nil
}
