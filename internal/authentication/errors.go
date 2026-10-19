package authentication

import (
	"errors"
)

var (
	ErrBadToken        = errors.New("invalid or expired token")
	ErrBadSignIn       = errors.New("incorrect email or password")
	ErrAlreadyVerified = errors.New("email already verified")
	ErrEmailTaken      = errors.New("email already registered")
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidPassword = errors.New("invalid password")
)
