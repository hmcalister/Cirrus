package config

import "errors"

// ServerConfig is the configuration used by cmd/server.
type ServerConfig struct {
	HTTP           HTTPConfig
	Database       DatabaseConfig
	Email          EmailConfig
	EmailRateLimit EmailRateLimitConfig
	Auth           AuthenticationConfig
	AuthHTTP       AuthHTTPConfig
	Debug          bool
}

// Read the configuration needed by cmd/server from the environment.
func LoadServer() (ServerConfig, error) {
	httpCfg, httpErr := loadHTTP()
	database, databaseErr := loadDatabase()
	email, emailErr := loadEmail()
	rateLimit, rateLimitErr := loadEmailRateLimit()
	auth, authErr := loadAuth()
	authHTTP, authHTTPErr := loadAuthHTTP()

	cfg := ServerConfig{
		HTTP:           httpCfg,
		Database:       database,
		Email:          email,
		EmailRateLimit: rateLimit,
		Auth:           auth,
		AuthHTTP:       authHTTP,
		Debug:          debugEnabled(),
	}

	return cfg, errors.Join(httpErr, databaseErr, emailErr, rateLimitErr, authErr, authHTTPErr)
}
