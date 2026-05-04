package redis

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/model"
)

func TestGetPostDetailWithCache(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rdb = redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	initResilience() // Initialize otter and breaker

	ctx := context.Background()
	postID := int64(123)
	key := "forum:post:123"

	expectedPost := &model.Post{
		ID:    postID,
		Title: "Redis Test",
	}
	data, _ := json.Marshal(expectedPost)
	s.Set(key, string(data))

	post, err := GetPostDetailWithCache(ctx, postID)
	assert.Nil(t, err)
	assert.Equal(t, expectedPost.Title, post.Title)
}

// TestSingleflightMerge verifies that concurrent requests for the same cache key
// are merged by singleflight into exactly 1 DB call.
func TestSingleflightMerge(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rdb = redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	initResilience()

	// Setup sqlmock — only 1 DB query should be executed
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	postID := int64(999)
	ctx := context.Background()

	// Expect exactly 1 DB call (singleflight merges all concurrent ones)
	mock.ExpectQuery("select .* from posts where id = ?").
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title"}).AddRow(postID, "Singleflight Post"))

	// Launch 10 concurrent goroutines calling GetPostDetailWithCache
	const concurrency = 10
	var wg sync.WaitGroup
	results := make([]*model.Post, concurrency)
	errs := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = GetPostDetailWithCache(ctx, postID)
		}(i)
	}
	wg.Wait()

	// All go routines should succeed
	for i := 0; i < concurrency; i++ {
		assert.Nil(t, errs[i], "goroutine %d should not error", i)
		assert.NotNil(t, results[i])
		assert.Equal(t, postID, results[i].ID)
		assert.Equal(t, "Singleflight Post", results[i].Title)
	}

	// Only 1 DB call should have been made
	assert.NoError(t, mock.ExpectationsWereMet())
}
