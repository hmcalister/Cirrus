package api

import (
	"errors"
	"net/http"

	"github.com/hmcalister/Cirrus/internal/authentication"
)

const (
	// The generic fragment used when rendering fails unexpectedly.
	genericErrorMessage = `<div class="message error">Something went wrong. Try again.</div>`

	passwordResetSentMessage  = "If an account exists for that email, a reset link has been sent."
	verificationResentMessage = "If an account exists for that email, a verification link has been sent."
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("not signed in")
)

// Maps an authentication error to the status and message shown to the user.
func authErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, authentication.ErrBadLogin):
		return http.StatusUnauthorized, "Incorrect email or password."
	case errors.Is(err, authentication.ErrUnverified):
		return http.StatusForbidden, "Verify your email before signing in."
	case errors.Is(err, authentication.ErrEmailTaken):
		return http.StatusConflict, "An account with that email already exists."
	case errors.Is(err, authentication.ErrAlreadyVerified):
		return http.StatusConflict, "That email is already verified."
	case errors.Is(err, authentication.ErrBadToken):
		return http.StatusBadRequest, "That link is invalid or has expired."
	case errors.Is(err, authentication.ErrInvalidEmail):
		return http.StatusBadRequest, "Enter a valid email address."
	case errors.Is(err, authentication.ErrInvalidPassword):
		return http.StatusBadRequest, "Enter a password."
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized, "Sign in to continue."
	default:
		return http.StatusInternalServerError, "Something went wrong. Try again."
	}
}
