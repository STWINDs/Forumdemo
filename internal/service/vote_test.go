package service

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	dao_redis "github.com/your-username/forum/internal/dao/redis"
)

func TestVoteForPost_Upvote(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	postID := int64(100)
	s.ZAdd(dao_redis.KeyPostTimeZSet, float64(time.Now().Unix()), "100")

	dir, err := VoteForPost(1, postID, 1)
	assert.Nil(t, err)
	assert.Equal(t, int8(1), dir)
}

func TestVoteForPost_Repeated(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	postID := int64(200)
	s.ZAdd(dao_redis.KeyPostTimeZSet, float64(time.Now().Unix()), "200")

	dir, err := VoteForPost(1, postID, 1)
	assert.Nil(t, err)
	assert.Equal(t, int8(1), dir)

	dir, err = VoteForPost(1, postID, 1)
	assert.Nil(t, err)
	assert.Equal(t, int8(0), dir)
}

func TestVoteForPost_Flip(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	postID := int64(400)
	s.ZAdd(dao_redis.KeyPostTimeZSet, float64(time.Now().Unix()), "400")

	dir, _ := VoteForPost(1, postID, 1)
	assert.Equal(t, int8(1), dir)

	dir, _ = VoteForPost(1, postID, -1)
	assert.Equal(t, int8(-1), dir)
}
