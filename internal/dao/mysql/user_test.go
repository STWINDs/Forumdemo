package mysql

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/your-username/forum/internal/model"
)

func TestCheckUserExist(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	db = sqlxDB

	// Test user exists
	mock.ExpectQuery("select count\\(id\\) from users where username = ?").
		WithArgs("exist_user").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	err = CheckUserExist("exist_user")
	assert.Equal(t, ErrorUserExist, err)

	// Test user does not exist
	mock.ExpectQuery("select count\\(id\\) from users where username = ?").
		WithArgs("new_user").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	err = CheckUserExist("new_user")
	assert.Nil(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	db = sqlxDB

	user := &model.User{
		Username: "testuser",
		Password: "password123",
		Email:    "test@example.com",
	}

	mock.ExpectExec("insert into users").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), user.Email).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = InsertUser(user)
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
	db = sqlxDB

	username := "loginuser"
	password := "correct_pass"
	encrypted := encryptPassword(password)

	user := &model.User{
		Username: username,
		Password: password,
	}

	// Success case
	mock.ExpectQuery("select id, username, password from users where username = ?").
		WithArgs(username).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password"}).
			AddRow(1, username, encrypted))

	err = Login(user)
	assert.Nil(t, err)

	// Wrong password case
	user.Password = "wrong_pass"
	mock.ExpectQuery("select id, username, password from users where username = ?").
		WithArgs(username).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password"}).
			AddRow(1, username, encrypted))

	err = Login(user)
	assert.Equal(t, ErrorInvalidPassword, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}
