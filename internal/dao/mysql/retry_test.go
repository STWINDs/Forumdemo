package mysql

import (
	"database/sql/driver"
	"errors"
	"io"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestIsTransientError(t *testing.T) {
	require.True(t, IsTransientError(driver.ErrBadConn))
	require.True(t, IsTransientError(io.EOF))
	require.True(t, IsTransientError(&mysqldriver.MySQLError{Number: 1290, Message: "read only"}))
	require.True(t, IsTransientError(errors.New("write: broken pipe")))
	require.False(t, IsTransientError(ErrConversationForbidden))
}
