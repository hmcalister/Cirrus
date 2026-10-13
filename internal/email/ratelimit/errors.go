package ratelimit

import "errors"

var (
	ErrRateLimited = errors.New("email sender rate limited")
)
