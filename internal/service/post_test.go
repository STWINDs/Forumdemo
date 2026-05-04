package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/your-username/forum/internal/dao/mysql"
	dao_redis "github.com/your-username/forum/internal/dao/redis"
	"github.com/your-username/forum/internal/model"
)

func TestCreatePost(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	mysql.SetDB(sqlxDB)

	// Setup miniredis for redis.CreatePost call
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dao_redis.SetRDB(redis.NewClient(&redis.Options{Addr: s.Addr()}))
	dao_redis.InitResilienceForTest()

	post := &model.Post{
		AuthorID:    1,
		Title:       "Service Test",
		Content:     "Content",
		CommunityID: 1,
	}

	mock.ExpectExec("insert into posts").
		WithArgs(post.AuthorID, post.Title, post.Content, post.CommunityID, post.PostType, post.VideoURL).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = CreatePost(post)
	assert.Nil(t, err)
	assert.Equal(t, int64(1), post.ID)
}

func TestGetPostByID(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Setup Redis
	dao_redis.SetRDB(redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	}))
	dao_redis.InitResilienceForTest()

	// Mock DB in case of cache miss
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	mysql.SetDB(sqlxDB)

	ctx := context.Background()
	postID := int64(1)

	// Mock DB return for fallback
	mock.ExpectQuery("select (.+) from posts where id = ?").
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title"}).AddRow(1, "Post 1"))

	post, err := GetPostByID(ctx, postID)
	assert.Nil(t, err)
	assert.Equal(t, "Post 1", post.Title)
}

func TestGetPostList(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "author_id", "title", "content", "community_id", "status", "create_time", "update_time"}).
		AddRow(1, 1, "Post 1", "Content 1", 1, 1, now, now).
		AddRow(2, 2, "Post 2", "Content 2", 1, 1, now, now)

	mock.ExpectQuery("select .* from posts order by create_time desc").
		WithArgs(int64(0), int64(10)).
		WillReturnRows(rows)

	posts, err := GetPostList(1, 10)
	assert.Nil(t, err)
	assert.Len(t, posts, 2)
	assert.Equal(t, "Post 1", posts[0].Title)
}
