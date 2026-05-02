package redis

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	oneWeekInSeconds = 7 * 24 * 3600
	scorePerVote     = 432 // 每一票的分数
)

var (
	ErrVoteTimeExpire = errors.New("投票时间已过")
	ErrVoteRepeated   = errors.New("不允许重复投票")
)

/* 投票逻辑：
direction=1时：
	1. 之前未投票 -> 赞成, 分数+432
	2. 之前投反对票 -> 改为赞成, 分数+432*2
direction=0时：
	1. 之前投赞成票 -> 取消投票, 分数-432
	2. 之前投反对票 -> 取消投票, 分数+432
direction=-1时：
	1. 之前未投票 -> 反对, 分数-432
	2. 之前投赞成票 -> 改为反对, 分数-432*2

记录用户投票记录：zset (key: KeyPostVotedPrefix+postID, member: userID, score: direction)
*/

func VoteForPost(userID, postID string, direction float64) error {
	ctx := context.Background()
	// 1. 判断投票限制
	// 去redis取帖子发布时间
	postTime := rdb.ZScore(ctx, KeyPostTimeZSet, postID).Val()
	if float64(time.Now().Unix())-postTime > oneWeekInSeconds {
		return ErrVoteTimeExpire
	}

	// 2. 更新分数
	// 先查当前用户之前的投票纪录
	key := KeyPostVotedPrefix + postID
	ov := rdb.ZScore(ctx, key, userID).Val()

	// 更新：如果这一次投票和之前保存的投票一致，就提示不允许重复投票
	if direction == ov {
		return ErrVoteRepeated
	}

	var op float64
	if direction > ov {
		op = 1
	} else {
		op = -1
	}
	diff := math.Abs(ov - direction) // 计算两次投票的差值

	pipeline := rdb.TxPipeline()
	pipeline.ZIncrBy(ctx, KeyPostScoreZSet, op*diff*scorePerVote, postID)

	// 3. 记录用户为该帖子投过票
	if direction == 0 {
		pipeline.ZRem(ctx, key, userID)
	} else {
		pipeline.ZAdd(ctx, key, &redis.Z{
			Score:  direction,
			Member: userID,
		})
	}
	_, err := pipeline.Exec(ctx)
	return err
}

func CreatePost(postID int64) error {
	ctx := context.Background()
	pipeline := rdb.TxPipeline()
	// 帖子发布时间
	pipeline.ZAdd(ctx, KeyPostTimeZSet, &redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: strconv.FormatInt(postID, 10),
	})
	// 帖子初始分数
	pipeline.ZAdd(ctx, KeyPostScoreZSet, &redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: strconv.FormatInt(postID, 10),
	})
	_, err := pipeline.Exec(ctx)
	return err
}
