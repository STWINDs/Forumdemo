package mysql

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/your-username/forum/config"
	"go.uber.org/zap"
)

var db *sqlx.DB

func SetDB(db_ *sqlx.DB) {
	db = db_
}

func Stats() sql.DBStats {
	if db == nil {
		return sql.DBStats{}
	}
	return db.Stats()
}

func Init(cfg *config.MySQLConfig) (err error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local&timeout=5s&readTimeout=10s&writeTimeout=10s&rejectReadOnly=true",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
	)
	db, err = sqlx.Connect("mysql", dsn)
	if err != nil {
		zap.L().Error("connect DB failed", zap.Error(err))
		return
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	// Router can move new connections to a newly elected Primary. Recycling
	// pooled connections bounds how long an old backend connection is retained.
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(time.Minute)
	return
}

func isReady() error {
	if db == nil {
		return ErrDBNotReady
	}
	return nil
}

func Close() {
	if db != nil {
		_ = db.Close()
	}
}
