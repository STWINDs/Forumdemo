package mysql

import (
	"database/sql/driver"
	"errors"
	"io"
	"net"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// IsTransientError identifies failures for which opening a new Router-backed
// connection and retrying an idempotent operation is safe. Callers must still
// provide their own idempotency boundary for writes.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1047, 1053, 1290, 2006, 2013:
			return true
		}
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{"broken pipe", "connection reset", "connection is already closed", "server has gone away", "read-only", "unexpected eof"} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}
