package redis

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
	"github.com/your-username/forum/internal/model"
)

const (
	directMessageRelationTTL     = 5 * time.Minute
	directMessageConversationTTL = 2 * time.Minute
	directMessageUnreadTTL       = 10 * time.Minute
	directMessageCacheVersionTTL = 24 * time.Hour
)

type conversationCacheMeta struct {
	Limit         int   `json:"limit"`
	Complete      bool  `json:"complete"`
	LastMessageID int64 `json:"last_message_id"`
}

func directMessageRelationKey(a, b int64) string {
	if a > b {
		a, b = b, a
	}
	return Prefix + "social:{relation:" + strconv.FormatInt(a, 10) + ":" + strconv.FormatInt(b, 10) + "}"
}

func directMessageUserKeyPrefix(userID int64) string {
	// Every key touched by one user's Lua scripts/transactions must share this
	// exact hash tag in Redis Cluster.
	return Prefix + "dm:{user:" + strconv.FormatInt(userID, 10) + "}:"
}

func directMessageConversationIndexKey(userID int64) string {
	return directMessageUserKeyPrefix(userID) + "conversations"
}

func directMessageConversationMetaKey(userID int64) string {
	return directMessageUserKeyPrefix(userID) + "conversations:meta"
}

func directMessageConversationVersionKey(userID int64) string {
	return directMessageUserKeyPrefix(userID) + "conversations:version"
}

func directMessageConversationSummaryKey(userID, conversationID int64) string {
	return directMessageUserKeyPrefix(userID) + "conversation:" + strconv.FormatInt(conversationID, 10)
}

func directMessageUnreadTotalKey(userID int64) string {
	return directMessageUserKeyPrefix(userID) + "unread_total"
}

func directMessageUnreadHashKey(userID int64) string {
	return directMessageUserKeyPrefix(userID) + "unread"
}

func GetCachedUserRelation(ctx context.Context, currentUserID, otherUserID int64) (*model.UserRelation, bool, error) {
	if err := isReady(); err != nil {
		return nil, false, err
	}
	value, err := rdb.Get(ctx, directMessageRelationKey(currentUserID, otherUserID)).Result()
	if err == redisv9.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var relation model.UserRelation
	if err := json.Unmarshal([]byte(value), &relation); err != nil {
		_ = rdb.Del(ctx, directMessageRelationKey(currentUserID, otherUserID)).Err()
		return nil, false, err
	}

	// The stored flags are oriented from the lower user ID. Reverse them when
	// the caller is the higher user so the response remains caller-relative.
	if currentUserID > otherUserID {
		relation.Following, relation.FollowedBy = relation.FollowedBy, relation.Following
	}
	return &relation, true, nil
}

func SetCachedUserRelation(ctx context.Context, currentUserID, otherUserID int64, relation *model.UserRelation) error {
	if err := isReady(); err != nil {
		return err
	}
	stored := *relation
	if currentUserID > otherUserID {
		stored.Following, stored.FollowedBy = stored.FollowedBy, stored.Following
	}
	value, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, directMessageRelationKey(currentUserID, otherUserID), value, directMessageRelationTTL).Err()
}

func InvalidateCachedUserRelation(ctx context.Context, currentUserID, otherUserID int64) error {
	if err := isReady(); err != nil {
		return err
	}
	return rdb.Del(ctx, directMessageRelationKey(currentUserID, otherUserID)).Err()
}

// GetCachedConversationList returns a cache hit only when the cached window is
// large enough for the requested limit and every referenced summary exists.
func GetCachedConversationList(ctx context.Context, userID int64, limit int) ([]*model.ConversationSummary, bool, error) {
	if err := isReady(); err != nil {
		return nil, false, err
	}

	metaValue, err := rdb.Get(ctx, directMessageConversationMetaKey(userID)).Result()
	if err == redisv9.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var meta conversationCacheMeta
	if err := json.Unmarshal([]byte(metaValue), &meta); err != nil {
		return nil, false, err
	}
	if !meta.Complete && meta.Limit < limit {
		return nil, false, nil
	}

	conversationIDs, err := rdb.ZRevRange(ctx, directMessageConversationIndexKey(userID), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, false, err
	}
	if len(conversationIDs) == 0 {
		return []*model.ConversationSummary{}, true, nil
	}

	pipeline := rdb.Pipeline()
	commands := make([]*redisv9.StringCmd, 0, len(conversationIDs))
	for _, idValue := range conversationIDs {
		conversationID, parseErr := strconv.ParseInt(idValue, 10, 64)
		if parseErr != nil {
			return nil, false, parseErr
		}
		commands = append(commands, pipeline.Get(ctx, directMessageConversationSummaryKey(userID, conversationID)))
	}
	if _, err := pipeline.Exec(ctx); err != nil && err != redisv9.Nil {
		return nil, false, err
	}

	items := make([]*model.ConversationSummary, 0, len(commands))
	for i := range commands {
		value, err := commands[i].Result()
		if err == redisv9.Nil {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		var item model.ConversationSummary
		if err := json.Unmarshal([]byte(value), &item); err != nil {
			return nil, false, err
		}
		items = append(items, &item)
	}
	return items, true, nil
}

func SetCachedConversationList(ctx context.Context, userID int64, requestedLimit int, items []*model.ConversationSummary) error {
	if err := isReady(); err != nil {
		return err
	}
	indexKey := directMessageConversationIndexKey(userID)
	metaKey := directMessageConversationMetaKey(userID)
	var lastMessageID int64
	for _, item := range items {
		if item.LastMessageID > lastMessageID {
			lastMessageID = item.LastMessageID
		}
	}
	metaValue, err := json.Marshal(conversationCacheMeta{
		Limit:         requestedLimit,
		Complete:      len(items) < requestedLimit,
		LastMessageID: lastMessageID,
	})
	if err != nil {
		return err
	}

	keys := []string{indexKey, metaKey, directMessageConversationVersionKey(userID), directMessageUnreadHashKey(userID)}
	args := []interface{}{lastMessageID, string(metaValue), requestedLimit, len(items)}
	for _, item := range items {
		value, err := json.Marshal(item)
		if err != nil {
			return err
		}
		// Milliseconds keep real-time order; the final three digits make an ID
		// tie-breaker for messages created in the same millisecond.
		score := float64(item.LastMessageAt.UnixMilli()*1000 + item.ConversationID%1000)
		keys = append(keys, directMessageConversationSummaryKey(userID, item.ConversationID))
		args = append(args, item.ConversationID, score, string(value), item.UnreadCount)
	}
	script := redisv9.NewScript(`
		local incoming = tonumber(ARGV[1]) or 0
		local watermark = tonumber(redis.call('GET', KEYS[3])) or 0
		if watermark > 0 and incoming < watermark then
			return 0
		end
		redis.call('DEL', KEYS[1], KEYS[4])
		redis.call('SET', KEYS[3], ARGV[1], 'EX', ` + strconv.FormatInt(int64(directMessageCacheVersionTTL/time.Second), 10) + `)
		local itemCount = tonumber(ARGV[4])
		local arg = 5
		for i = 1, itemCount do
			local conversationID = tostring(ARGV[arg])
			local summaryKey = KEYS[4 + i]
			redis.call('ZADD', KEYS[1], ARGV[arg + 1], conversationID)
			redis.call('SET', summaryKey, ARGV[arg + 2], 'EX', ` + strconv.FormatInt(int64(directMessageConversationTTL/time.Second), 10) + `)
			redis.call('HSET', KEYS[4], conversationID, ARGV[arg + 3])
			arg = arg + 4
		end
		if itemCount > 0 then
			redis.call('EXPIRE', KEYS[1], ` + strconv.FormatInt(int64(directMessageConversationTTL/time.Second), 10) + `)
			redis.call('EXPIRE', KEYS[4], ` + strconv.FormatInt(int64(directMessageUnreadTTL/time.Second), 10) + `)
		end
		redis.call('SET', KEYS[2], ARGV[2], 'EX', ` + strconv.FormatInt(int64(directMessageConversationTTL/time.Second), 10) + `)
		return 1
	`)
	_, err = script.Run(ctx, rdb, keys, args...).Result()
	return err
}

func InvalidateCachedConversationLists(ctx context.Context, conversationID int64, userIDs ...int64) error {
	return invalidateCachedConversationListsAtVersion(ctx, 0, conversationID, userIDs...)
}

func InvalidateCachedConversationListsAtVersion(ctx context.Context, lastMessageID, conversationID int64, userIDs ...int64) error {
	return invalidateCachedConversationListsAtVersion(ctx, lastMessageID, conversationID, userIDs...)
}

func invalidateCachedConversationListsAtVersion(ctx context.Context, lastMessageID, conversationID int64, userIDs ...int64) error {
	if err := isReady(); err != nil {
		return err
	}
	if lastMessageID <= 0 {
		for _, userID := range userIDs {
			// Different users intentionally live in different slots. DEL is
			// atomic for each user and Outbox retries make the multi-user
			// invalidation eventually complete.
			if err := rdb.Del(ctx,
				directMessageConversationIndexKey(userID),
				directMessageConversationMetaKey(userID),
				directMessageUnreadHashKey(userID),
				directMessageConversationSummaryKey(userID, conversationID),
			).Err(); err != nil {
				return err
			}
		}
		return nil
	}

	// A delayed outbox event must never lower the per-user watermark or delete
	// cache built for a newer message. Compare-and-invalidate is one Redis
	// operation so no refill can interleave between the version check and DEL.
	script := redisv9.NewScript(`
		local incoming = tonumber(ARGV[1]) or 0
		local watermark = tonumber(redis.call('GET', KEYS[1])) or 0
		if watermark > 0 and incoming < watermark then
			return 0
		end
		redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
		redis.call('DEL', KEYS[2], KEYS[3], KEYS[4], KEYS[5])
		return 1
	`)
	for _, userID := range userIDs {
		_, err := script.Run(ctx, rdb, []string{
			directMessageConversationVersionKey(userID),
			directMessageConversationIndexKey(userID),
			directMessageConversationMetaKey(userID),
			directMessageUnreadHashKey(userID),
			directMessageConversationSummaryKey(userID, conversationID),
		}, lastMessageID, int64(directMessageCacheVersionTTL/time.Second)).Result()
		if err != nil {
			return err
		}
	}
	return nil
}

func GetCachedDirectMessageUnreadTotal(ctx context.Context, userID int64) (int64, bool, error) {
	if err := isReady(); err != nil {
		return 0, false, err
	}
	value, err := rdb.Get(ctx, directMessageUnreadTotalKey(userID)).Result()
	if err == redisv9.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	count, err := strconv.ParseInt(value, 10, 64)
	return count, err == nil, err
}

func SetCachedDirectMessageUnreadTotal(ctx context.Context, userID, count int64) error {
	if err := isReady(); err != nil {
		return err
	}
	return rdb.Set(ctx, directMessageUnreadTotalKey(userID), count, directMessageUnreadTTL).Err()
}

// IncrementDirectMessageUnreadIfCached never creates a counter from an
// unknown baseline. If either cache is absent, the next read rebuilds it from
// MySQL instead of producing a plausible but incorrect value.
func IncrementDirectMessageUnreadIfCached(ctx context.Context, userID, conversationID int64) error {
	if err := isReady(); err != nil {
		return err
	}
	script := redisv9.NewScript(`
		if redis.call('EXISTS', KEYS[1]) == 1 then
			redis.call('INCR', KEYS[1])
		end
		if redis.call('EXISTS', KEYS[2]) == 1 then
			redis.call('HINCRBY', KEYS[2], ARGV[1], 1)
		end
		return 1
	`)
	_, err := script.Run(ctx, rdb,
		[]string{directMessageUnreadTotalKey(userID), directMessageUnreadHashKey(userID)},
		strconv.FormatInt(conversationID, 10),
	).Result()
	return err
}

func InvalidateDirectMessageUnreadCache(ctx context.Context, userID int64) error {
	if err := isReady(); err != nil {
		return err
	}
	return rdb.Del(ctx, directMessageUnreadTotalKey(userID), directMessageUnreadHashKey(userID)).Err()
}
