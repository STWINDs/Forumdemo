package service

import (
	"strconv"

	"github.com/your-username/forum/internal/dao/redis"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/pkg/kafka"
)

// VoteForPost 帖子投票 — Redis 同步 + Kafka 异步
// 返回实际生效的投票方向: 1, -1
func VoteForPost(userID, postID int64, direction int8) (int8, error) {
	uid := strconv.FormatInt(userID, 10)
	pid := strconv.FormatInt(postID, 10)

	actual, err := redis.VoteForPost(uid, pid, float64(direction))
	if err != nil {
		return 0, err
	}

	// 异步投递 Kafka，不阻塞返回
	vote := &model.Vote{
		UserID:    userID,
		PostID:    postID,
		Direction: int8(actual),
	}
	kafka.EnqueueEvent("vote", vote)

	return int8(actual), nil
}

// VoteForComment 评论投票 — Redis 同步 + Kafka 异步
func VoteForComment(userID, commentID int64, direction int8) (int8, error) {
	uid := strconv.FormatInt(userID, 10)
	cid := strconv.FormatInt(commentID, 10)

	actual, err := redis.VoteForComment(uid, cid, float64(direction))
	if err != nil {
		return 0, err
	}

	cv := &model.CommentVote{
		UserID:    userID,
		CommentID: commentID,
		Direction: int8(actual),
	}
	kafka.EnqueueEvent("comment_vote", cv)

	return int8(actual), nil
}

// PostVoteInfo 帖子投票信息（赞数 + 用户投态）
type PostVoteInfo struct {
	Upvotes   int64 `json:"upvotes"`
	Downvotes int64 `json:"downvotes"`
	MyVote    int8  `json:"my_vote"` // 1, -1, or 0
}

// GetPostVoteInfo 获取帖子的赞数 + 当前用户的投票
func GetPostVoteInfo(postID, userID int64) (*PostVoteInfo, error) {
	pid := strconv.FormatInt(postID, 10)
	uid := strconv.FormatInt(userID, 10)

	up, down, err := redis.GetPostVoteCounts(pid)
	if err != nil {
		return nil, err
	}

	myVote, _ := redis.GetUserPostVote(uid, pid)

	return &PostVoteInfo{
		Upvotes:   up,
		Downvotes: down,
		MyVote:    int8(myVote),
	}, nil
}

// GetPostVoteCountsBatch 批量获取帖子赞数（帖子列表用）
func GetPostVoteCountsBatch(postIDs []int64) (map[int64][]int64, error) {
	return redis.PipelineGetPostVoteCounts(postIDs)
}

// GetCommentVoteInfo 批量获取评论的赞数 + 当前用户投态
func GetCommentVoteInfo(userID int64, commentIDs []int64) (map[int64][]int64, map[int64]int8, error) {
	uid := strconv.FormatInt(userID, 10)
	cids := make([]string, len(commentIDs))
	for i, id := range commentIDs {
		cids[i] = strconv.FormatInt(id, 10)
	}

	counts, err := redis.GetCommentVoteCounts(cids)
	if err != nil {
		return nil, nil, err
	}

	userVotes, err := redis.GetUserCommentVotes(uid, cids)
	if err != nil {
		return nil, nil, err
	}

	// Convert string keys back to int64
	resultCounts := make(map[int64][]int64, len(counts))
	for i, cid := range commentIDs {
		if v, ok := counts[cids[i]]; ok {
			resultCounts[cid] = v
		}
	}

	resultVotes := make(map[int64]int8, len(userVotes))
	for i, cid := range commentIDs {
		if v, ok := userVotes[cids[i]]; ok {
			resultVotes[cid] = int8(v)
		}
	}

	return resultCounts, resultVotes, nil
}
