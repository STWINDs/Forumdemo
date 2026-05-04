package mysql

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
	"github.com/your-username/forum/config"
	"go.uber.org/zap"
)

var db *sqlx.DB

func SetDB(db_ *sqlx.DB) {
	db = db_
}

func Init(cfg *config.MySQLConfig) (err error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
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
