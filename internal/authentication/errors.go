package authentication

import (
	"errors"
)

var (
	ErrBadToken        = errors.New("invalid or expired token")
	ErrBadLogin        = errors.New("incorrect email or password")
	ErrUnverified      = errors.New("email not verified")
	ErrAlreadyVerified = errors.New("email already verified")
	ErrEmailTaken      = errors.New("email already registered")
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidPassword = errors.New("invalid password")
)
