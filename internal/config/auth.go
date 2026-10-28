package config

import "time"

type AuthenticationConfig struct {
	// How long a magic sign-in link remains valid.
	LinkExpiryDuration time.Duration
	// How long a signed-in session remains valid.
	SessionExpiryDuration time.Duration
}

type AuthHTTPConfig struct {
	BaseURL string
}

// How often magic link requests may be made before stratus refuses them.
type AuthRateLimitConfig struct {
	// How many link requests one address may make immediately, and how long it takes to restore one.
	EmailBurst    int
	EmailInterval time.Duration

	// How many link requests one client IP may make immediately, and how long it takes to restore one.
	IPBurst    int
	IPInterval time.Duration

	// Answers denied link requests with 429 and Retry-After instead of the uniform 202.
	RevealDenials bool
}

// Read the magic link request rate limits, which only the server applies.
func loadAuthRateLimit() (AuthRateLimitConfig, error) {
	var l loader

	cfg := AuthRateLimitConfig{
		EmailBurst:    l.int(envAuthRateLimitEmailBurst),
		EmailInterval: l.duration(envAuthRateLimitEmailInterval),
		IPBurst:       l.int(envAuthRateLimitIPBurst),
		IPInterval:    l.duration(envAuthRateLimitIPInterval),
		RevealDenials: l.bool(envAuthRateLimitRevealDenials),
	}

	return cfg, l.err()
}

// Read the authentication configuration, which only the server uses.
func loadAuth() (AuthenticationConfig, error) {
	var l loader

	cfg := AuthenticationConfig{
		LinkExpiryDuration:    l.duration(envAuthLinkExpiry),
		SessionExpiryDuration: l.duration(envAuthSessionExpiry),
	}

	return cfg, l.err()
}

func loadAuthHTTP() (AuthHTTPConfig, error) {
	var l loader

	cfg := AuthHTTPConfig{
		BaseURL: l.string(envAuthBaseURL),
	}

	return cfg, l.err()
}
