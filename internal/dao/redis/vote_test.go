package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

func TestVoteForPost_Toggle(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rdb = redis.NewClient(&redis.Options{Addr: s.Addr()})
	initResilience()

	userID := "1"
	postID := "100"
	now := float64(time.Now().Unix())
	s.ZAdd(KeyPostTimeZSet, now, postID)

	// Test 1: First upvote
	dir, err := VoteForPost(userID, postID, 1)
	assert.Nil(t, err)
	assert.Equal(t, float64(1), dir)
	voted, _ := s.ZScore(KeyPostVotedPrefix+postID, userID)
	assert.Equal(t, float64(1), voted)

	up, down, _ := GetPostVoteCounts(postID)
	assert.Equal(t, int64(1), up)
	assert.Equal(t, int64(0), down)

	// Test 2: Same vote again = cancel
	dir, err = VoteForPost(userID, postID, 1)
	assert.Nil(t, err)
	assert.Equal(t, float64(0), dir)

	up, down, _ = GetPostVoteCounts(postID)
	assert.Equal(t, int64(0), up)
	assert.Equal(t, int64(0), down)

	// Test 3: Flip to downvote
	dir, err = VoteForPost(userID, postID, -1)
	assert.Nil(t, err)
	assert.Equal(t, float64(-1), dir)
	voted, _ = s.ZScore(KeyPostVotedPrefix+postID, userID)
	assert.Equal(t, float64(-1), voted)

	up, down, _ = GetPostVoteCounts(postID)
	assert.Equal(t, int64(0), up)
	assert.Equal(t, int64(1), down)
}

func TestVoteForComment_Toggle(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rdb = redis.NewClient(&redis.Options{Addr: s.Addr()})
	initResilience()

	userID := "1"
	commentID := "42"

	// New upvote
	dir, err := VoteForComment(userID, commentID, 1)
	assert.Nil(t, err)
	assert.Equal(t, float64(1), dir)

	voted, _ := s.ZScore(KeyCommentVotedPrefix+commentID, userID)
	assert.Equal(t, float64(1), voted)

	// Flip to downvote
	dir, err = VoteForComment(userID, commentID, -1)
	assert.Nil(t, err)
	assert.Equal(t, float64(-1), dir)
}

func TestGetPostVoteCounts(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rdb = redis.NewClient(&redis.Options{Addr: s.Addr()})
	initResilience()

	// Seed vote counts
	rdb.HMSet(ctx, KeyPostVoteHash+"100", "up", 5, "down", 2)

	up, down, err := GetPostVoteCounts("100")
	assert.Nil(t, err)
	assert.Equal(t, int64(5), up)
	assert.Equal(t, int64(2), down)
}

func TestBatchVoteCounts(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rdb = redis.NewClient(&redis.Options{Addr: s.Addr()})
	initResilience()

	rdb.HMSet(ctx, KeyPostVoteHash+"1", "up", 10, "down", 3)
	rdb.HMSet(ctx, KeyPostVoteHash+"2", "up", 0, "down", 1)

	counts, err := PipelineGetPostVoteCounts([]int64{1, 2})
	assert.Nil(t, err)
	assert.Equal(t, int64(10), counts[1][0])
	assert.Equal(t, int64(3), counts[1][1])
	assert.Equal(t, int64(0), counts[2][0])
	assert.Equal(t, int64(1), counts[2][1])
}

func TestCreatePostRedis(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rdb = redis.NewClient(&redis.Options{Addr: s.Addr()})
	initResilience()

	err = CreatePost(200)
	assert.Nil(t, err)

	// Check vote counts seeded
	up, down, _ := GetPostVoteCounts("200")
	assert.Equal(t, int64(0), up)
	assert.Equal(t, int64(0), down)
}
