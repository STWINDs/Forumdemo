package service

import (
	"context"
	"sort"

	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/dao/redis"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/pkg/kafka"
)

func CreateComment(comment *model.Comment) (err error) {
	return kafka.SendEvent(context.Background(), "comment", comment)
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
