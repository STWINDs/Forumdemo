package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenToken(t *testing.T) {
	token, err := GenToken(1, "testuser")
	assert.Nil(t, err)
	assert.NotEmpty(t, token)
}

func TestParseToken(t *testing.T) {
	token, err := GenToken(42, "alice")
	assert.Nil(t, err)

	claims, err := ParseToken(token)
	assert.Nil(t, err)
	assert.Equal(t, int64(42), claims.UserID)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, "forum", claims.Issuer)
}

func TestParseToken_Invalid(t *testing.T) {
	_, err := ParseToken("invalid.token.here")
	assert.NotNil(t, err)

	_, err = ParseToken("")
	assert.NotNil(t, err)
}

func TestGenToken_DifferentUsers(t *testing.T) {
	t1, _ := GenToken(1, "a")
	t2, _ := GenToken(2, "b")
	assert.NotEqual(t, t1, t2)

	c1, _ := ParseToken(t1)
	c2, _ := ParseToken(t2)
	assert.Equal(t, int64(1), c1.UserID)
	assert.Equal(t, int64(2), c2.UserID)
}

func TestTokenExpiration(t *testing.T) {
	token, err := GenToken(1, "user")
	assert.Nil(t, err)

	claims, err := ParseToken(token)
	assert.Nil(t, err)
	assert.True(t, claims.ExpiresAt.Time.After(time.Now()))
}
