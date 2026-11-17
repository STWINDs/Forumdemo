package service

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/dao/redis"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/pkg/kafka"
)

var ErrInvalidComment = errors.New("invalid comment: content 1..2000 characters, positive post ID and valid request ID required")
var commentRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var commentFingerprint = regexp.MustCompile(`^[a-f0-9]{64}$`)

func CreateComment(comment *model.Comment) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return SubmitComment(ctx, comment)
}

func SubmitComment(ctx context.Context, c *model.Comment) error {
	c.Content = strings.TrimSpace(c.Content)
	if c.ClientCommentID == "" {
		c.ClientCommentID = uuid.NewString()
	}
	if c.AuthorID <= 0 || c.PostID <= 0 || c.ParentID < 0 || c.Content == "" || !utf8.ValidString(c.Content) || utf8.RuneCountInString(c.Content) > 2000 || !commentRequestID.MatchString(c.ClientCommentID) {
		return ErrInvalidComment
	}
	// The HTTP caller receives a receipt only after synchronous broker acknowledgement.
	c.ID, c.Status = 0, 0
	c.CreateTime = time.Now().UTC()
	c.UpdateTime = c.CreateTime
	return kafka.SendEvent(ctx, "comment", c)
}

func GetCommentSubmission(authorID int64, clientID string) (*model.CommentSubmission, error) {
	return GetCommentSubmissionForPayload(authorID, clientID, "")
}

func GetCommentSubmissionForPayload(authorID int64, clientID, fingerprint string) (*model.CommentSubmission, error) {
	if !commentRequestID.MatchString(clientID) || (fingerprint != "" && !commentFingerprint.MatchString(fingerprint)) {
		return nil, ErrInvalidComment
	}
	c, err := mysql.GetCommentSubmission(authorID, clientID)
	if err != nil {
		return nil, err
	}
	if c == nil || (fingerprint != "" && c.Fingerprint() != fingerprint) {
		if fingerprint != "" {
			failed, err := mysql.HasCommentSubmissionFailure(authorID, clientID, fingerprint)
			if err != nil {
				return nil, err
			}
			if failed {
				return &model.CommentSubmission{State: "failed"}, nil
			}
		}
		return &model.CommentSubmission{State: "processing"}, nil
	}
	state := "published"
	if c.Status != 1 {
		state = "unavailable"
	}
	return &model.CommentSubmission{State: state, Comment: c}, nil
}

func GetCommentsByPostID(postID int64) (comments []*model.Comment, err error) {
	return mysql.GetCommentsByPostID(postID)
}

// CommentWithVotes 评论 + 投票信息
type CommentWithVotes struct {
	*model.Comment
	Upvotes   int64 `json:"upvotes"`
	Downvotes int64 `json:"downvotes"`
	MyVote    int8  `json:"my_vote"` // 1, -1, or 0
}

// GetCommentsWithVotes 获取评论并附带投票数和排序
func GetCommentsWithVotes(postID int64, sortBy string, userID int64) ([]CommentWithVotes, error) {
	comments, err := mysql.GetCommentsByPostID(postID)
	if err != nil {
		return nil, err
	}

	if len(comments) == 0 {
		return []CommentWithVotes{}, nil
	}

	ids := make([]int64, len(comments))
	cidStrs := make([]string, len(comments))
	for i, c := range comments {
		ids[i] = c.ID
		cidStrs[i] = formatInt64(c.ID)
	}

	// Batch fetch vote counts and user votes from Redis
	counts, _ := redis.GetCommentVoteCounts(cidStrs)
	userVotes, _ := redis.GetUserCommentVotes(formatInt64(userID), cidStrs)

	result := make([]CommentWithVotes, len(comments))
	for i, c := range comments {
		up, down := int64(0), int64(0)
		if vc, ok := counts[cidStrs[i]]; ok && len(vc) >= 2 {
			up, down = vc[0], vc[1]
		}
		mv := int8(0)
		if v, ok := userVotes[cidStrs[i]]; ok {
			mv = int8(v)
		}
		result[i] = CommentWithVotes{
			Comment:   c,
			Upvotes:   up,
			Downvotes: down,
			MyVote:    mv,
		}
	}

	// Sort
	if sortBy == "new" {
		sort.SliceStable(result, func(i, j int) bool {
			return result[i].CreateTime.After(result[j].CreateTime)
		})
	} else {
		// "hot": sort by (upvotes - downvotes) desc
		sort.SliceStable(result, func(i, j int) bool {
			si := result[i].Upvotes - result[i].Downvotes
			sj := result[j].Upvotes - result[j].Downvotes
			return si > sj
		})
	}

	return result, nil
}

func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
