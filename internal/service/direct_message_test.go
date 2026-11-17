package service

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/your-username/forum/internal/dao/mysql"
	daoRedis "github.com/your-username/forum/internal/dao/redis"
)

func TestFollowUserPersistsWhenRedisIsUnavailable(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = mockDB.Close() })
	mysql.SetDB(sqlx.NewDb(mockDB, "mysql"))
	daoRedis.SetRDB(nil)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users WHERE id = \?`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO follows`).
		WithArgs(int64(1), int64(2)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = FollowUser(context.Background(), 1, 2)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRetryTransientMySQL(t *testing.T) {
	original := transientRetryDelays
	transientRetryDelays = []time.Duration{time.Millisecond}
	t.Cleanup(func() { transientRetryDelays = original })

	attempts := 0
	err := retryTransientMySQL(context.Background(), func() error {
		attempts++
		if attempts == 1 {
			return io.EOF
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, attempts)

	attempts = 0
	want := errors.New("business rule")
	err = retryTransientMySQL(context.Background(), func() error {
		attempts++
		return want
	})
	require.ErrorIs(t, err, want)
	require.Equal(t, 1, attempts)
}
