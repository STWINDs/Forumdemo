package mysql

import "errors"

var (
	ErrorUserExist       = errors.New("user already exists")
	ErrorUserNotExist    = errors.New("user does not exist")
	ErrorInvalidPassword = errors.New("invalid password")
	ErrDBNotReady        = errors.New("database not connected")

	ErrCannotMessageSelf      = errors.New("cannot message yourself")
	ErrConversationNotFound   = errors.New("conversation not found")
	ErrConversationForbidden  = errors.New("user is not a member of this conversation")
	ErrWaitingForReply        = errors.New("wait for the other user to reply before sending again")
	ErrStrangerMessageLimit   = errors.New("stranger message limit reached")
	ErrDuplicateClientMessage = errors.New("client message id already exists")
	ErrOutboxLeaseLost        = errors.New("outbox processing lease lost")
)
