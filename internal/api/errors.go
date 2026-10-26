package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hmcalister/stratus/pkg/auth"
)

const (
	// The generic fragment used when rendering fails unexpectedly.
	genericErrorMessage = `<div class="message error">Something went wrong. Try again.</div>`
)

var (
	ErrInvalidInput = errors.New("invalid input")
)

// Maps an authentication error to the status and message shown to the user.
func authErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, auth.ErrRateLimited):
		return http.StatusTooManyRequests, "Too many sign-in links requested. Try again later"
	case errors.Is(err, auth.ErrInvalidEmail):
		return http.StatusBadRequest, "Enter a valid email address"
	case errors.Is(err, auth.ErrUnauthenticated):
		return http.StatusUnauthorized, "Sign in to continue"
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest, err.Error()
	default:
		return http.StatusInternalServerError, fmt.Sprintf("Something went wrong. Error %s", err)
	}
}
