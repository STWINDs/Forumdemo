package redis

import "fmt"

const (
	Prefix = "forum:"
	// The two global indexes share a hash tag so they can be updated in one
	// Cluster transaction.
	KeyPostTimeZSet  = Prefix + "post:{index}:time"
	KeyPostScoreZSet = Prefix + "post:{index}:score"

	// Deprecated legacy prefixes are retained for source compatibility only.
	// New code must use the key builders below so related keys share a slot.
	KeyPostVotedPrefix    = Prefix + "post:voted:"
	KeyPostVoteHash       = Prefix + "post:votes:"
	KeyCommentVotedPrefix = Prefix + "comment:voted:"
	KeyCommentVoteHash    = Prefix + "comment:votes:"
	KeyPostCommentCount   = Prefix + "post:comment_count:"
)

func PostVotedKey(postID string) string {
	return fmt.Sprintf("%spost:{%s}:voted", Prefix, postID)
}

func PostVoteCountKey(postID string) string {
	return fmt.Sprintf("%spost:{%s}:votes", Prefix, postID)
}

func PostCommentCountKey(postID string) string {
	return fmt.Sprintf("%spost:{%s}:comment_count", Prefix, postID)
}

func CommentVotedKey(commentID string) string {
	return fmt.Sprintf("%scomment:{%s}:voted", Prefix, commentID)
}

func CommentVoteCountKey(commentID string) string {
	return fmt.Sprintf("%scomment:{%s}:votes", Prefix, commentID)
}

func getRedisKey(key string) string {
	return key
}
