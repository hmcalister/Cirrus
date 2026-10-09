// Package config loads application configuration from the environment.

package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"time"
)

type Config struct {
	Addr              string
	DatabaseURL       string
	Debug             bool
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

func Load() (Config, error) {
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(getRequiredEnv("POSTGRES_USER"), getRequiredEnv("POSTGRES_PASSWORD")),
		Host:   getRequiredEnv("POSTGRES_HOST") + ":" + getRequiredEnv("POSTGRES_PORT"),
		Path:   "/" + getRequiredEnv("POSTGRES_DB"),
	}

	return Config{
		Addr:              getRequiredEnv("HTTP_ADDR"),
		DatabaseURL:       dsn.String(),
		Debug:             os.Getenv("DEBUG") != "",
		ReadHeaderTimeout: toDurationStrict(getRequiredEnv("HTTP_READ_HEADER_TIMEOUT")),
		ReadTimeout:       toDurationStrict(getRequiredEnv("HTTP_READ_TIMEOUT")),
		WriteTimeout:      toDurationStrict(getRequiredEnv("HTTP_WRITE_TIMEOUT")),
		IdleTimeout:       toDurationStrict(getRequiredEnv("HTTP_IDLE_TIMEOUT")),
		ShutdownTimeout:   toDurationStrict(getRequiredEnv("HTTP_SHUTDOWN_TIMEOUT")),
		MaxConns:          toInt32Strict(getRequiredEnv("DB_MAX_CONNS")),
		MinConns:          toInt32Strict(getRequiredEnv("DB_MIN_CONNS")),
		MaxConnLifetime:   toDurationStrict(getRequiredEnv("DB_MAX_CONN_LIFETIME")),
		MaxConnIdleTime:   toDurationStrict(getRequiredEnv("DB_MAX_CONN_IDLE_TIME")),
		HealthCheckPeriod: toDurationStrict(getRequiredEnv("DB_HEALTH_CHECK_PERIOD")),
	}, nil
}

// Loads an environment variable associated with the given key, or panics.
func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		slog.Error("required environment variable is not set", "variable", key)
		panic("required environment variable not set")
	}
	return value
}

func toDurationStrict(durationStr string) time.Duration {
	d, err := time.ParseDuration(durationStr)
	if err != nil {
		slog.Error("string could not be parsed to duration", "durationStr", durationStr)
		panic("invalid duration found in strict parsing")
	}
	return d
}

func toInt32Strict(intStr string) int32 {
	var i int32
	n, err := fmt.Sscanf(intStr, "%d", &i)
	if err != nil || n != 1 {
		slog.Error("string could not be parsed to int32", "intStr", intStr)
		panic("invalid int32 found in strict parsing")
	}
	return i
}
