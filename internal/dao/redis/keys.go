package redis

const (
	Prefix             = "forum:"
	KeyPostTimeZSet    = Prefix + "post:time"
	KeyPostScoreZSet   = Prefix + "post:score"
	KeyPostVotedPrefix = Prefix + "post:voted:" // zset per post: member=userID, score=±1

	// Vote counts (hash per post/comment for fast batch reads)
	KeyPostVoteHash    = Prefix + "post:votes:"    // hash: {up, down}
	KeyCommentVotedPrefix = Prefix + "comment:voted:" // zset per comment
	KeyCommentVoteHash = Prefix + "comment:votes:"   // hash: {up, down}

	// Comment count per post
	KeyPostCommentCount = Prefix + "post:comment_count:"
)

func getRedisKey(key string) string {
	return key
}
