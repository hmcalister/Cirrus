// Package config reads the application configuration from the environment.
//
// Each executable loads only the configuration it uses, so a missing variable
// stops the process that needs it rather than its sibling.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Environment variable names. Centralised here so that a rename is a single edit and so that the names can be reused by tests.
const (
	envDebug = "DEBUG"

	envHTTPPort              = "HTTP_PORT"
	envHTTPReadHeaderTimeout = "HTTP_READ_HEADER_TIMEOUT"
	envHTTPReadTimeout       = "HTTP_READ_TIMEOUT"
	envHTTPWriteTimeout      = "HTTP_WRITE_TIMEOUT"
	envHTTPIdleTimeout       = "HTTP_IDLE_TIMEOUT"
	envHTTPShutdownTimeout   = "HTTP_SHUTDOWN_TIMEOUT"

	envPostgresUser     = "POSTGRES_USER"
	envPostgresPassword = "POSTGRES_PASSWORD"
	envPostgresHost     = "POSTGRES_HOST"
	envPostgresPort     = "POSTGRES_PORT"
	envPostgresDB       = "POSTGRES_DB"

	envDBMaxConns          = "DB_MAX_CONNS"
	envDBMinConns          = "DB_MIN_CONNS"
	envDBMaxConnLifetime   = "DB_MAX_CONN_LIFETIME"
	envDBMaxConnIdleTime   = "DB_MAX_CONN_IDLE_TIME"
	envDBHealthCheckPeriod = "DB_HEALTH_CHECK_PERIOD"

	envEmailSMTPHost     = "EMAIL_SMTP_HOST"
	envEmailSMTPPort     = "EMAIL_SMTP_PORT"
	envEmailSMTPUsername = "EMAIL_SMTP_USERNAME"
	envEmailSMTPPassword = "EMAIL_SMTP_PASSWORD"
	envEmailFromAddress  = "EMAIL_FROM_ADDRESS"
	envEmailFromName     = "EMAIL_FROM_NAME"
	envEmailSMTPTimeout  = "EMAIL_SMTP_TIMEOUT"

	envEmailRateLimitAddressBurst         = "EMAIL_RATE_LIMIT_ADDRESS_BURST"
	envEmailRateLimitAddressInterval      = "EMAIL_RATE_LIMIT_ADDRESS_INTERVAL"
	envEmailRateLimitGlobalBurst          = "EMAIL_RATE_LIMIT_GLOBAL_BURST"
	envEmailRateLimitGlobalInterval       = "EMAIL_RATE_LIMIT_GLOBAL_INTERVAL"
	envEmailRateLimitSweepCadenceDuration = "EMAIL_RATE_LIMIT_SWEEP_CADENCE"

	envAuthLinkExpiry    = "AUTH_LINK_EXPIRY"
	envAuthSessionExpiry = "AUTH_SESSION_EXPIRY"
	envAuthBaseURL       = "AUTH_BASE_URL"

	envAuthRateLimitEmailBurst    = "AUTH_RATE_LIMIT_EMAIL_BURST"
	envAuthRateLimitEmailInterval = "AUTH_RATE_LIMIT_EMAIL_INTERVAL"
	envAuthRateLimitIPBurst       = "AUTH_RATE_LIMIT_IP_BURST"
	envAuthRateLimitIPInterval    = "AUTH_RATE_LIMIT_IP_INTERVAL"
	envAuthRateLimitRevealDenials = "AUTH_RATE_LIMIT_REVEAL_DENIALS"

	envDailyEmailTime         = "DAILY_EMAIL_TIME"
	envDailyEmailBCCBatchSize = "DAILY_EMAIL_BCC_BATCH_SIZE"

	envS3Bucket   = "S3_BUCKET"
	envS3Region   = "S3_REGION"
	envS3Endpoint = "S3_ENDPOINT"
)

// loader reads fields from the environment and collects the problems it finds, so that every problem can be reported at once.
type loader struct {
	errs []error
}

func (l *loader) string(key string) string {
	value, err := requiredEnv(key)
	if err != nil {
		l.errs = append(l.errs, err)
	}
	return value
}

func (l *loader) duration(key string) time.Duration {
	value, err := requiredDuration(key)
	if err != nil {
		l.errs = append(l.errs, err)
	}
	return value
}

func (l *loader) int32(key string) int32 {
	value, err := requiredInt32(key)
	if err != nil {
		l.errs = append(l.errs, err)
	}
	return value
}

func (l *loader) int(key string) int {
	return int(l.int32(key))
}

func (l *loader) bool(key string) bool {
	value, err := requiredBool(key)
	if err != nil {
		l.errs = append(l.errs, err)
	}
	return value
}

func (l *loader) clock(key string) (int, int) {
	hour, minute, err := requiredClockTime(key)
	if err != nil {
		l.errs = append(l.errs, err)
	}
	return hour, minute
}

// Returns every problem found while loading, or nil.
func (l *loader) err() error {
	return errors.Join(l.errs...)
}

// Debug logging is on when DEBUG is set to anything.
func debugEnabled() bool {
	return os.Getenv(envDebug) != ""
}

func requiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return value, nil
}

func requiredDuration(key string) (time.Duration, error) {
	value, err := requiredEnv(key)
	if err != nil {
		return 0, err
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("environment variable %s is not a duration: %w", key, err)
	}
	return d, nil
}

func requiredInt32(key string) (int32, error) {
	value, err := requiredEnv(key)
	if err != nil {
		return 0, err
	}
	i, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("environment variable %s is not an int32: %w", key, err)
	}
	return int32(i), nil
}

func requiredBool(key string) (bool, error) {
	value, err := requiredEnv(key)
	if err != nil {
		return false, err
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("environment variable %s is not a bool: %w", key, err)
	}
	return b, nil
}

// Parse a time of day given as HH:MM and return its hour and minute.
func requiredClockTime(key string) (int, int, error) {
	value, err := requiredEnv(key)
	if err != nil {
		return 0, 0, err
	}
	t, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, fmt.Errorf("environment variable %s is not a time of day (HH:MM): %w", key, err)
	}
	return t.Hour(), t.Minute(), nil
}
