package service

import (
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/pkg/jwt"
)

func SignUp(p *model.User) (err error) {
	if err = mysql.CheckUserExist(p.Username); err != nil {
		return err
	}
	return mysql.InsertUser(p)
}

func Login(p *model.User) (token string, err error) {
	if err = mysql.Login(p); err != nil {
		return "", err
	}
	return jwt.GenToken(p.ID, p.Username)
}
