package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func TestGetWithResilience(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rdb = redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	initResilience()

	ctx := context.Background()
	key := "test_resilience"
	type target struct {
		Name string `json:"name"`
	}

	// 1. Test: L1 Miss, L2 Miss, DB Fetch Success
	expected := &target{Name: "DB Value"}
	fetchCount := 0
	dbFetch := func() (interface{}, error) {
		fetchCount++
		return expected, nil
	}

	var res target
	err = GetWithResilience(ctx, key, &res, time.Minute, dbFetch)
	assert.Nil(t, err)
	assert.Equal(t, expected.Name, res.Name)
	assert.Equal(t, 1, fetchCount)

	// Wait for async L2 write (since it's in a goroutine in cache.go)
	time.Sleep(100 * time.Millisecond)

	// 2. Test: L1 Hit
	fetchCount = 0
	err = GetWithResilience(ctx, key, &res, time.Minute, dbFetch)
	assert.Nil(t, err)
	assert.Equal(t, 0, fetchCount)

	// 3. Test: L1 Miss (Delete from L1), L2 Hit
	DelL1(key)
	err = GetWithResilience(ctx, key, &res, time.Minute, dbFetch)
	assert.Nil(t, err)
	assert.Equal(t, 0, fetchCount)

	// 4. Test: DB Fetch Failure
	keyErr := "test_error"
	dbFetchErr := func() (interface{}, error) {
		return nil, errors.New("db error")
	}
	err = GetWithResilience(ctx, keyErr, &res, time.Minute, dbFetchErr)
	assert.NotNil(t, err)
	assert.Equal(t, "db error", err.Error())
}
