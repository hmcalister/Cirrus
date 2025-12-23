package token

import (
	"errors"
	"net/http"
	"strings"
)

var (
	ErrInvalidBearerToken = errors.New("invalid or missing bearer token")
)

type AuthTokenManager interface {
	// Create a token with the given claims and return it.
	// The returned token should be ready for transmission across the public internet,
	// i.e. it should be signed and/or encrypted.
	CreateToken(claims map[string]interface{}) (token string)

	// Verify the given token (which can be assumed to be from the CreateToken method)
	// Returns encrypted claims and true if token is valid,
	// or an empty map and false if the token is invalid.
	VerifyToken(token string) (claims map[string]interface{}, ok bool)
}

func GetTokenFromRequestHeader(header http.Header) (token string, err error) {
	authHeader := header.Get("Authorization")
	if authHeader == "" {
		return "", ErrInvalidBearerToken
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", ErrInvalidBearerToken
	}
	return parts[1], nil
}
