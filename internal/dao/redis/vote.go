package redis

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

/* Toggle voting logic:
   ov = user's current vote: 0 (none), 1 (up), -1 (down)
   direction = new vote: 1 (up) or -1 (down)

   if direction == ov → cancel (remove vote, decr count)
   elif ov == 0       → new vote (add to zset, incr count)
   else               → flip vote (update zset, incr new + decr old)
*/

func VoteForPost(userID, postID string, direction float64) (float64, error) {
	if err := isReady(); err != nil {
		return 0, err
	}
	ctx := context.Background()

	key := PostVotedKey(postID)
	ov := rdb.ZScore(ctx, key, userID).Val()
	return toggleVote(ctx, key, PostVoteCountKey(postID), userID, ov, direction)
}

func VoteForComment(userID, commentID string, direction float64) (float64, error) {
	if err := isReady(); err != nil {
		return 0, err
	}
	ctx := context.Background()

	key := CommentVotedKey(commentID)
	ov := rdb.ZScore(ctx, key, userID).Val()
	return toggleVote(ctx, key, CommentVoteCountKey(commentID), userID, ov, direction)
}

func toggleVote(ctx context.Context, votedKey, countKey, userID string, oldVote, direction float64) (float64, error) {
	pipeline := rdb.TxPipeline()

	if direction == oldVote {
		// Cancel vote
		pipeline.ZRem(ctx, votedKey, userID)
		if direction == 1 {
			pipeline.HIncrBy(ctx, countKey, "up", -1)
		} else {
			pipeline.HIncrBy(ctx, countKey, "down", -1)
		}
		_, err := pipeline.Exec(ctx)
		return 0, err
	}

	if oldVote == 0 {
		pipeline.ZAdd(ctx, votedKey, redis.Z{Score: direction, Member: userID})
		if direction == 1 {
			pipeline.HIncrBy(ctx, countKey, "up", 1)
		} else {
			pipeline.HIncrBy(ctx, countKey, "down", 1)
		}
	} else {
		pipeline.ZAdd(ctx, votedKey, redis.Z{Score: direction, Member: userID})
		if direction == 1 {
			pipeline.HIncrBy(ctx, countKey, "up", 1)
			pipeline.HIncrBy(ctx, countKey, "down", -1)
		} else {
			pipeline.HIncrBy(ctx, countKey, "down", 1)
			pipeline.HIncrBy(ctx, countKey, "up", -1)
		}
	}

	_, err := pipeline.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return direction, nil
}

// GetPostVoteCounts returns {up, down} from hash
func GetPostVoteCounts(postID string) (up, down int64, err error) {
	if err = isReady(); err != nil {
		return 0, 0, err
	}
	ctx := context.Background()
	results, err := rdb.HMGet(ctx, PostVoteCountKey(postID), "up", "down").Result()
	if err != nil {
		return 0, 0, err
	}
	if results[0] != nil {
		up, _ = strconv.ParseInt(results[0].(string), 10, 64)
	}
	if results[1] != nil {
		down, _ = strconv.ParseInt(results[1].(string), 10, 64)
	}
	return up, down, nil
}

// GetUserPostVote returns the user's vote on a post: 1, -1, or 0
func GetUserPostVote(userID, postID string) (float64, error) {
	if err := isReady(); err != nil {
		return 0, err
	}
	ctx := context.Background()
	return rdb.ZScore(ctx, PostVotedKey(postID), userID).Val(), nil
}

// GetCommentVoteCounts batch-fetches vote counts for multiple comments
// Returns map[commentID] → {up, down}
func GetCommentVoteCounts(commentIDs []string) (map[string][]int64, error) {
	if err := isReady(); err != nil {
		return nil, err
	}
	if len(commentIDs) == 0 {
		return map[string][]int64{}, nil
	}

	ctx := context.Background()
	pipeline := rdb.Pipeline()
	for _, cid := range commentIDs {
		pipeline.HMGet(ctx, CommentVoteCountKey(cid), "up", "down")
	}
	cmds, err := pipeline.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	result := make(map[string][]int64, len(commentIDs))
	for i, cmd := range cmds {
		vals, _ := cmd.(*redis.SliceCmd).Result()
		var up, down int64
		if len(vals) > 0 && vals[0] != nil {
			up, _ = strconv.ParseInt(vals[0].(string), 10, 64)
		}
		if len(vals) > 1 && vals[1] != nil {
			down, _ = strconv.ParseInt(vals[1].(string), 10, 64)
		}
		result[commentIDs[i]] = []int64{up, down}
	}
	return result, nil
}

// GetUserCommentVotes batch-fetches the current user's vote on multiple comments
func GetUserCommentVotes(userID string, commentIDs []string) (map[string]float64, error) {
	if err := isReady(); err != nil {
		return nil, err
	}
	if len(commentIDs) == 0 {
		return map[string]float64{}, nil
	}

	ctx := context.Background()
	pipeline := rdb.Pipeline()
	for _, cid := range commentIDs {
		pipeline.ZScore(ctx, CommentVotedKey(cid), userID)
	}
	cmds, err := pipeline.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	result := make(map[string]float64, len(commentIDs))
	for i, cmd := range cmds {
		val := cmd.(*redis.FloatCmd).Val()
		result[commentIDs[i]] = val
	}
	return result, nil
}

// PipelineGetPostVoteCounts batch-fetches up/down counts for multiple posts
func PipelineGetPostVoteCounts(postIDs []int64) (map[int64][]int64, error) {
	if err := isReady(); err != nil {
		return nil, err
	}
	if len(postIDs) == 0 {
		return map[int64][]int64{}, nil
	}

	ctx := context.Background()
	pipeline := rdb.Pipeline()
	for _, pid := range postIDs {
		pipeline.HMGet(ctx, PostVoteCountKey(strconv.FormatInt(pid, 10)), "up", "down")
	}
	cmds, err := pipeline.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	result := make(map[int64][]int64, len(postIDs))
	for i, cmd := range cmds {
		vals, _ := cmd.(*redis.SliceCmd).Result()
		var up, down int64
		if len(vals) > 0 && vals[0] != nil {
			up, _ = strconv.ParseInt(vals[0].(string), 10, 64)
		}
		if len(vals) > 1 && vals[1] != nil {
			down, _ = strconv.ParseInt(vals[1].(string), 10, 64)
		}
		result[postIDs[i]] = []int64{up, down}
	}
	return result, nil
}

// InitPostVoteCounts seeds up/down hash for a newly created post
func InitPostVoteCounts(postID int64) error {
	if err := isReady(); err != nil {
		return err
	}
	ctx := context.Background()
	return rdb.HMSet(ctx, PostVoteCountKey(strconv.FormatInt(postID, 10)),
		"up", 0, "down", 0).Err()
}

// CreatePost seeds the post into time/score zsets and initializes vote counts
func CreatePost(postID int64) error {
	if err := isReady(); err != nil {
		return err
	}
	ctx := context.Background()
	now := float64(time.Now().Unix())
	pidStr := strconv.FormatInt(postID, 10)

	// The per-post vote hash belongs to the post slot. Initialize it before
	// publishing the post into the separate global-index slot.
	if err := rdb.HMSet(ctx, PostVoteCountKey(pidStr), "up", 0, "down", 0).Err(); err != nil {
		return err
	}
	_, err := rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.ZAdd(ctx, KeyPostTimeZSet, redis.Z{Score: now, Member: pidStr})
		p.ZAdd(ctx, KeyPostScoreZSet, redis.Z{Score: now, Member: pidStr})
		return nil
	})
	return err
}
