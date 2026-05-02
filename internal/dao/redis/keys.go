package redis

const (
	Prefix             = "forum:"
	KeyPostTimeZSet    = Prefix + "post:time"   // zset: 帖子发帖时间
	KeyPostScoreZSet   = Prefix + "post:score"  // zset: 帖子分数
	KeyPostVotedPrefix = Prefix + "post:voted:" // set: 记录用户投票类型;参数是post_id
)

func getRedisKey(key string) string {
	return key
}
