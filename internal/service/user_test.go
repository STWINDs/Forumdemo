package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/model"
)

func TestSignUp(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	mysql.SetDB(sqlxDB)

	p := &model.User{
		Username: "new_user",
		Password: "password123",
		Email:    "new@example.com",
	}

	// 1. Check user exist (not exist)
	mock.ExpectQuery("select count\\(id\\) from users where username = ?").
		WithArgs(p.Username).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// 2. Insert user
	mock.ExpectExec("insert into users").
		WithArgs(p.Username, sqlmock.AnyArg(), p.Email).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = SignUp(p)
	assert.Nil(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	mysql.SetDB(sqlxDB)

	p := &model.User{
		Username: "login_user",
		Password: "password123",
	}

	// Mocking mysql.Login success
	// We need the encrypted password for the mock return
	// Since encryptPassword is private, we can't easily get it here,
	// but we know it uses MD5 with secret "forum"

	// Actually, let's just mock what mysql.Login expects
	mock.ExpectQuery("select id, username, password from users where username = ?").
		WithArgs(p.Username).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password"}).
			AddRow(1, p.Username, "ebb673aee573ae069156c6a731098300"))

	token, err := Login(p)
	assert.Nil(t, err)
	assert.NotEmpty(t, token)
}
