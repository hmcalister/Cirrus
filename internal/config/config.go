package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Environment variable names. Centralised here so that a rename is a single
// edit and so that the names can be reused by tests.
const (
	envDebug = "DEBUG"

	envHTTPAddr              = "HTTP_ADDR"
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
)

type DatabaseConfig struct {
	DatabaseURL       string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

type ServerConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type EmailConfig struct {
	Host        string
	Port        string
	Username    string
	Password    string
	FromAddress string
	FromName    string
	Timeout     time.Duration
}

type EmailRateLimitConfig struct {
	AddressBurst         int
	AddressInterval      time.Duration
	GlobalBurst          int
	GlobalInterval       time.Duration
	SweepCadenceDuration time.Duration
}

type AuthenticationConfig struct {
	// How long a magic sign-in link remains valid.
	LinkExpiryDuration time.Duration
	// How long a signed-in session remains valid.
	SessionExpiryDuration time.Duration
}

type AuthHTTPConfig struct {
	BaseURL string
}

type Config struct {
	Server         ServerConfig
	Database       DatabaseConfig
	Email          EmailConfig
	EmailRateLimit EmailRateLimitConfig
	Auth           AuthenticationConfig
	AuthHTTP       AuthHTTPConfig
	Debug          bool
}

// Read the application configuration from the environment.
//
// Every problem found while reading the environment is collected and returned.
func Load() (Config, error) {
	var errs []error

	require_string := func(key string) string {
		value, err := requiredEnv(key)
		if err != nil {
			errs = append(errs, err)
		}
		return value
	}
	require_duration := func(key string) time.Duration {
		value, err := requiredDuration(key)
		if err != nil {
			errs = append(errs, err)
		}
		return value
	}
	require_int32 := func(key string) int32 {
		value, err := requiredInt32(key)
		if err != nil {
			errs = append(errs, err)
		}
		return value
	}
	require_int := func(key string) int {
		return int(require_int32(key))
	}

	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(require_string(envPostgresUser), require_string(envPostgresPassword)),
		Host:   require_string(envPostgresHost) + ":" + require_string(envPostgresPort),
		Path:   "/" + require_string(envPostgresDB),
	}

	cfg := Config{
		Server: ServerConfig{
			Addr:              require_string(envHTTPAddr),
			ReadHeaderTimeout: require_duration(envHTTPReadHeaderTimeout),
			ReadTimeout:       require_duration(envHTTPReadTimeout),
			WriteTimeout:      require_duration(envHTTPWriteTimeout),
			IdleTimeout:       require_duration(envHTTPIdleTimeout),
			ShutdownTimeout:   require_duration(envHTTPShutdownTimeout),
		},
		Database: DatabaseConfig{
			DatabaseURL:       dsn.String(),
			MaxConns:          require_int32(envDBMaxConns),
			MinConns:          require_int32(envDBMinConns),
			MaxConnLifetime:   require_duration(envDBMaxConnLifetime),
			MaxConnIdleTime:   require_duration(envDBMaxConnIdleTime),
			HealthCheckPeriod: require_duration(envDBHealthCheckPeriod),
		},
		Email: EmailConfig{
			Host:        require_string(envEmailSMTPHost),
			Port:        require_string(envEmailSMTPPort),
			Username:    require_string(envEmailSMTPUsername),
			Password:    require_string(envEmailSMTPPassword),
			FromAddress: require_string(envEmailFromAddress),
			FromName:    require_string(envEmailFromName),
			Timeout:     require_duration(envEmailSMTPTimeout),
		},
		EmailRateLimit: EmailRateLimitConfig{
			AddressBurst:         require_int(envEmailRateLimitAddressBurst),
			AddressInterval:      require_duration(envEmailRateLimitAddressInterval),
			GlobalBurst:          require_int(envEmailRateLimitGlobalBurst),
			GlobalInterval:       require_duration(envEmailRateLimitGlobalInterval),
			SweepCadenceDuration: require_duration(envEmailRateLimitSweepCadenceDuration),
		},
		Auth: AuthenticationConfig{
			LinkExpiryDuration:    require_duration(envAuthLinkExpiry),
			SessionExpiryDuration: require_duration(envAuthSessionExpiry),
		},
		AuthHTTP: AuthHTTPConfig{
			BaseURL: require_string(envAuthBaseURL),
		},
		Debug: os.Getenv(envDebug) != "",
	}

	return cfg, errors.Join(errs...)
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
