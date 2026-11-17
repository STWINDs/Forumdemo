package mysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func TestGetDirectMessageOutboxStats(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = mockDB.Close() })
	SetDB(sqlx.NewDb(mockDB, "mysql"))

	mock.ExpectQuery(`SELECT[\s\S]+FROM direct_message_outbox WHERE status IN`).
		WillReturnRows(sqlmock.NewRows([]string{"pending", "processing", "oldest"}).AddRow(7, 1, 12))
	pending, processing, oldest, err := GetDirectMessageOutboxStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(7), pending)
	require.Equal(t, int64(1), processing)
	require.Equal(t, int64(12), oldest)
	require.NoError(t, mock.ExpectationsWereMet())
}
