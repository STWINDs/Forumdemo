package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/your-username/forum/internal/model"
)

func CreateComment(c *model.Comment) (err error) {
	if err = isReady(); err != nil {
		return
	}
	if c.ClientCommentID != "" {
		return createCommentOnce(c)
	}
	sqlStr := `insert into comments(content, author_id, post_id, parent_id) values(?,?,?,?)`
	_, err = db.Exec(sqlStr, c.Content, c.AuthorID, c.PostID, c.ParentID)
	return
}

var ErrCommentIDConflict = errors.New("comment request ID already used with different content")

func RecordCommentSubmissionFailure(ctx context.Context, c *model.Comment) error {
	if err := isReady(); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `INSERT INTO comment_submission_failures(author_id, client_comment_id, fingerprint)
 VALUES(?,?,?) ON DUPLICATE KEY UPDATE fingerprint=fingerprint`, c.AuthorID, c.ClientCommentID, c.Fingerprint())
	return err
}

func HasCommentSubmissionFailure(authorID int64, clientID, fingerprint string) (bool, error) {
	var failed bool
	if err := isReady(); err != nil {
		return false, err
	}
	err := db.Get(&failed, `SELECT EXISTS(SELECT 1 FROM comment_submission_failures WHERE author_id=? AND client_comment_id=? AND fingerprint=?)`, authorID, clientID, fingerprint)
	return failed, err
}

// The unique receipt serializes concurrent replays. The comment and receipt
// commit together, including when the first commit succeeded but its reply was lost.
func createCommentOnce(c *model.Comment) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT INTO comment_submissions(author_id, client_comment_id, comment_id) VALUES(?,?,NULL)
		ON DUPLICATE KEY UPDATE client_comment_id = client_comment_id`, c.AuthorID, c.ClientCommentID)
	if err != nil {
		return err
	}
	var id sql.NullInt64
	if err = tx.Get(&id, `SELECT comment_id FROM comment_submissions WHERE author_id=? AND client_comment_id=? FOR UPDATE`, c.AuthorID, c.ClientCommentID); err != nil {
		return err
	}
	if id.Valid {
		var existing model.Comment
		if err = tx.Get(&existing, `SELECT * FROM comments WHERE id=?`, id.Int64); err != nil {
			return err
		}
		if existing.Content != c.Content || existing.PostID != c.PostID || existing.ParentID != c.ParentID {
			return ErrCommentIDConflict
		}
		c.ID = id.Int64
		return tx.Commit()
	}
	result, err := tx.Exec(`insert into comments(content, author_id, post_id, parent_id) values(?,?,?,?)`, c.Content, c.AuthorID, c.PostID, c.ParentID)
	if err != nil {
		return err
	}
	c.ID, err = result.LastInsertId()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE comment_submissions SET comment_id=? WHERE author_id=? AND client_comment_id=?`, c.ID, c.AuthorID, c.ClientCommentID); err != nil {
		return err
	}
	return tx.Commit()
}

func GetCommentSubmission(authorID int64, clientID string) (*model.Comment, error) {
	if err := isReady(); err != nil {
		return nil, err
	}
	var c model.Comment
	err := db.Get(&c, `SELECT c.* FROM comments c JOIN comment_submissions s ON s.comment_id=c.id WHERE s.author_id=? AND s.client_comment_id=?`, authorID, clientID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query comment submission: %w", err)
	}
	c.ClientCommentID = clientID
	return &c, nil
}

func GetCommentsByPostID(postID int64) (comments []*model.Comment, err error) {
	if err = isReady(); err != nil {
		return
	}
	sqlStr := `select id, content, author_id, post_id, parent_id, status, create_time, update_time
			   from comments where post_id = ? and status = 1 order by create_time desc`
	comments = make([]*model.Comment, 0, 10)
	err = db.Select(&comments, sqlStr, postID)
	return
}
