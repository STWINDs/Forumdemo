package mysql

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/your-username/forum/internal/model"
)

const secret = "forum"

func CheckUserExist(username string) (err error) {
	if err = isReady(); err != nil { return }
	sqlStr := `select count(id) from users where username = ?`
	var count int
	if err := db.Get(&count, sqlStr, username); err != nil {
		return err
	}
	if count > 0 {
		return ErrorUserExist
	}
	return
}

func InsertUser(user *model.User) (err error) {
	if err = isReady(); err != nil { return }
	user.Password = encryptPassword(user.Password)
	sqlStr := `insert into users(username, password, email) values(?,?,?)`
	_, err = db.Exec(sqlStr, user.Username, user.Password, user.Email)
	return
}

func encryptPassword(oPassword string) string {
	h := md5.New()
	h.Write([]byte(secret))
	h.Write([]byte(oPassword))
	return hex.EncodeToString(h.Sum(nil))
}

func Login(user *model.User) (err error) {
	if err = isReady(); err != nil { return }
	oPassword := user.Password
	sqlStr := `select id, username, password from users where username = ?`
	err = db.Get(user, sqlStr, user.Username)
	if err != nil {
		return err
	}
	password := encryptPassword(oPassword)
	if password != user.Password {
		return ErrorInvalidPassword
	}
	return
}
