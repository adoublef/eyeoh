package errors

import "errors"

var (
	ErrInvalid    = errors.New("invalid argument")
	ErrPermission = errors.New("permission denied")
	ErrExist      = errors.New("already exists")
	ErrNotExist   = errors.New("does not exist")
	ErrClosed     = errors.New("already closed")
)
