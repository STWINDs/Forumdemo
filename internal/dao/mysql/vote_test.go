package mysql

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/your-username/forum/internal/model"
)

func TestCreateVote(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "mysql")
	db = sqlxDB

	vote := &model.Vote{
		UserID:    1,
		PostID:    1,
		Direction: 1,
	}

	mock.ExpectExec("INSERT INTO votes").
		WithArgs(vote.UserID, vote.PostID, vote.Direction, vote.Direction).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = CreateVote(vote)
	assert.Nil(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
