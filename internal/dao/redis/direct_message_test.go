package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redisv9 "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/your-username/forum/internal/model"
)

func setupDirectMessageRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server, err := miniredis.Run()
	require.NoError(t, err)
	rdb = redisv9.NewClient(&redisv9.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		server.Close()
	})
	return server
}

func TestDirectMessageRelationCacheKeepsCallerDirection(t *testing.T) {
	setupDirectMessageRedis(t)
	ctx := context.Background()

	err := SetCachedUserRelation(ctx, 20, 10, &model.UserRelation{
		Following:  true,
		FollowedBy: false,
		Mutual:     false,
	})
	require.NoError(t, err)

	fromTwenty, found, err := GetCachedUserRelation(ctx, 20, 10)
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, fromTwenty.Following)
	assert.False(t, fromTwenty.FollowedBy)

	fromTen, found, err := GetCachedUserRelation(ctx, 10, 20)
	require.NoError(t, err)
	require.True(t, found)
	assert.False(t, fromTen.Following)
	assert.True(t, fromTen.FollowedBy)

	require.NoError(t, InvalidateCachedUserRelation(ctx, 10, 20))
	_, found, err = GetCachedUserRelation(ctx, 20, 10)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestDirectMessageConversationListCache(t *testing.T) {
	setupDirectMessageRedis(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Millisecond)
	items := []*model.ConversationSummary{
		{ConversationID: 1, PeerID: 2, PeerUsername: "alice", LastMessage: "older", LastMessageAt: now.Add(-time.Minute), UnreadCount: 1},
		{ConversationID: 2, PeerID: 3, PeerUsername: "bob", LastMessage: "newer", LastMessageAt: now, UnreadCount: 4},
	}

	require.NoError(t, SetCachedConversationList(ctx, 1, 30, items))
	cached, found, err := GetCachedConversationList(ctx, 1, 30)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, cached, 2)
	assert.Equal(t, int64(2), cached[0].ConversationID)
	assert.Equal(t, int64(4), cached[0].UnreadCount)

	require.NoError(t, InvalidateCachedConversationLists(ctx, 2, 1))
	_, found, err = GetCachedConversationList(ctx, 1, 30)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestConversationListCacheRejectsOlderVersion(t *testing.T) {
	setupDirectMessageRedis(t)
	ctx := context.Background()

	require.NoError(t, SetCachedConversationList(ctx, 1, 30, []*model.ConversationSummary{
		{ConversationID: 1, LastMessageID: 20, LastMessage: "new"},
	}))
	require.NoError(t, SetCachedConversationList(ctx, 1, 30, []*model.ConversationSummary{
		{ConversationID: 1, LastMessageID: 10, LastMessage: "old"},
	}))

	cached, found, err := GetCachedConversationList(ctx, 1, 30)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, cached, 1)
	assert.Equal(t, "new", cached[0].LastMessage)
}

func TestConversationListCacheAllowsEqualVersionAfterInvalidation(t *testing.T) {
	setupDirectMessageRedis(t)
	ctx := context.Background()
	items := []*model.ConversationSummary{
		{ConversationID: 1, LastMessageID: 20, LastMessage: "current", LastMessageAt: time.Now()},
	}

	require.NoError(t, InvalidateCachedConversationListsAtVersion(ctx, 20, 1, 9))
	require.NoError(t, SetCachedConversationList(ctx, 9, 30, items))
	cached, found, err := GetCachedConversationList(ctx, 9, 30)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, cached, 1)
	assert.Equal(t, "current", cached[0].LastMessage)
}

func TestOlderInvalidationCannotLowerConversationWatermark(t *testing.T) {
	setupDirectMessageRedis(t)
	ctx := context.Background()
	require.NoError(t, SetCachedConversationList(ctx, 9, 30, []*model.ConversationSummary{
		{ConversationID: 1, LastMessageID: 21, LastMessage: "newest", LastMessageAt: time.Now()},
	}))

	require.NoError(t, InvalidateCachedConversationListsAtVersion(ctx, 19, 1, 9))
	require.NoError(t, SetCachedConversationList(ctx, 9, 30, []*model.ConversationSummary{
		{ConversationID: 1, LastMessageID: 20, LastMessage: "stale", LastMessageAt: time.Now()},
	}))

	cached, found, err := GetCachedConversationList(ctx, 9, 30)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, cached, 1)
	assert.Equal(t, "newest", cached[0].LastMessage)
}

func TestUnreadIncrementRequiresCachedBaseline(t *testing.T) {
	server := setupDirectMessageRedis(t)
	ctx := context.Background()

	// A missing cache must remain missing; starting from zero could undercount
	// unread messages already stored in MySQL.
	require.NoError(t, IncrementDirectMessageUnreadIfCached(ctx, 9, 100))
	assert.False(t, server.Exists(directMessageUnreadTotalKey(9)))
	assert.False(t, server.Exists(directMessageUnreadHashKey(9)))

	require.NoError(t, SetCachedDirectMessageUnreadTotal(ctx, 9, 5))
	require.NoError(t, SetCachedConversationList(ctx, 9, 30, []*model.ConversationSummary{
		{ConversationID: 100, LastMessageAt: time.Now(), UnreadCount: 2},
	}))
	require.NoError(t, IncrementDirectMessageUnreadIfCached(ctx, 9, 100))

	total, found, err := GetCachedDirectMessageUnreadTotal(ctx, 9)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int64(6), total)
	unread, err := rdb.HGet(ctx, directMessageUnreadHashKey(9), "100").Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(3), unread)
}
