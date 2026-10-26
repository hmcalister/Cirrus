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
