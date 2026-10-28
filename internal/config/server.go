package config

import "errors"

// ServerConfig is the configuration used by cmd/server.
type ServerConfig struct {
	HTTP           HTTPConfig
	Database       DatabaseConfig
	Email          EmailConfig
	EmailRateLimit EmailRateLimitConfig
	Auth           AuthenticationConfig
	AuthRateLimit  AuthRateLimitConfig
	AuthHTTP       AuthHTTPConfig
	DailyEmailTime DailyEmailTimeConfig
	S3             S3Config
	Debug          bool
}

// Read the configuration needed by cmd/server from the environment.
func LoadServer() (ServerConfig, error) {
	httpCfg, httpErr := loadHTTP()
	database, databaseErr := loadDatabase()
	email, emailErr := loadEmail()
	rateLimit, rateLimitErr := loadEmailRateLimit()
	auth, authErr := loadAuth()
	authRateLimit, authRateLimitErr := loadAuthRateLimit()
	authHTTP, authHTTPErr := loadAuthHTTP()
	dailyEmailTime, dailyEmailTimeErr := loadDailyEmailTime()
	s3, s3Err := loadS3()

	cfg := ServerConfig{
		HTTP:           httpCfg,
		Database:       database,
		Email:          email,
		EmailRateLimit: rateLimit,
		Auth:           auth,
		AuthRateLimit:  authRateLimit,
		AuthHTTP:       authHTTP,
		DailyEmailTime: dailyEmailTime,
		S3:             s3,
		Debug:          debugEnabled(),
	}

	return cfg, errors.Join(httpErr, databaseErr, emailErr, rateLimitErr, authErr, authRateLimitErr, authHTTPErr, dailyEmailTimeErr, s3Err)
}
