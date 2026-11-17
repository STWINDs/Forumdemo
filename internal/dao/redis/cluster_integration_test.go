package redis

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/model"
)

// TestRedisClusterCompatibility is opt-in because it needs a real Cluster;
// miniredis cannot validate slot routing, MOVED replies, or CROSSSLOT errors.
func TestRedisClusterCompatibility(t *testing.T) {
	raw := os.Getenv("REDIS_CLUSTER_TEST_ADDRS")
	if raw == "" {
		t.Skip("set REDIS_CLUSTER_TEST_ADDRS to run the Redis Cluster integration test")
	}

	ctx := context.Background()
	previous := rdb
	require.NoError(t, Init(&config.RedisConfig{
		Mode:     "cluster",
		Addrs:    strings.Split(raw, ","),
		PoolSize: 10,
	}))
	t.Cleanup(func() {
		Close()
		rdb = previous
	})

	seed := time.Now().UnixNano()
	userA := seed%1_000_000_000 + 1_000_000_000
	userB := userA + 1
	conversationID := userA + 100
	messageID := userA + 200
	postID := strconv.FormatInt(userA+300, 10)
	commentA := strconv.FormatInt(userA+400, 10)
	commentB := strconv.FormatInt(userA+401, 10)

	items := []*model.ConversationSummary{{
		ConversationID: conversationID,
		LastMessageID:  messageID,
		LastMessage:    "cluster-message",
		LastMessageAt:  time.Now(),
		UnreadCount:    1,
	}}
	require.NoError(t, SetCachedConversationList(ctx, userA, 30, items))
	cached, found, err := GetCachedConversationList(ctx, userA, 30)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, cached, 1)
	assert.Equal(t, "cluster-message", cached[0].LastMessage)

	// This exercises two different user slots. The implementation must issue
	// one atomic operation per user instead of a cross-slot transaction.
	require.NoError(t, InvalidateCachedConversationListsAtVersion(
		ctx, messageID, conversationID, userA, userB,
	))

	_, err = VoteForPost(strconv.FormatInt(userA, 10), postID, 1)
	require.NoError(t, err)
	up, down, err := GetPostVoteCounts(postID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), up)
	assert.Equal(t, int64(0), down)

	require.NoError(t, rdb.HMSet(ctx, CommentVoteCountKey(commentA), "up", 2, "down", 0).Err())
	require.NoError(t, rdb.HMSet(ctx, CommentVoteCountKey(commentB), "up", 0, "down", 3).Err())
	counts, err := GetCommentVoteCounts([]string{commentA, commentB})
	require.NoError(t, err)
	assert.Equal(t, []int64{2, 0}, counts[commentA])
	assert.Equal(t, []int64{0, 3}, counts[commentB])
}

func TestClusterKeyFamiliesUseOneHashTag(t *testing.T) {
	assert.Equal(t, "user:42", redisHashTag(directMessageConversationIndexKey(42)))
	assert.Equal(t, "user:42", redisHashTag(directMessageConversationVersionKey(42)))
	assert.Equal(t, "user:42", redisHashTag(directMessageUnreadHashKey(42)))
	assert.Equal(t, "42", redisHashTag(PostVotedKey("42")))
	assert.Equal(t, "42", redisHashTag(PostVoteCountKey("42")))
	assert.Equal(t, "42", redisHashTag(PostCommentCountKey("42")))
	assert.Equal(t, "99", redisHashTag(CommentVotedKey("99")))
	assert.Equal(t, "99", redisHashTag(CommentVoteCountKey("99")))
	assert.Equal(t, "index", redisHashTag(KeyPostTimeZSet))
	assert.Equal(t, "index", redisHashTag(KeyPostScoreZSet))
}

func redisHashTag(key string) string {
	start := strings.IndexByte(key, '{')
	if start < 0 {
		return key
	}
	rest := key[start+1:]
	end := strings.IndexByte(rest, '}')
	if end <= 0 {
		return key
	}
	return rest[:end]
}
