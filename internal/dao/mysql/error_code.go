package mysql

import "errors"

var (
	ErrorUserExist       = errors.New("user already exists")
	ErrorUserNotExist    = errors.New("user does not exist")
	ErrorInvalidPassword = errors.New("invalid password")
	ErrDBNotReady        = errors.New("database not connected")
)
