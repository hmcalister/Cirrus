package config

import (
	"net/url"
	"time"
)

type DatabaseConfig struct {
	DatabaseURL       string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// Read the database configuration shared by every executable.
func loadDatabase() (DatabaseConfig, error) {
	var l loader

	// The URL pieces are read once because the DSN is built from them.
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(l.string(envPostgresUser), l.string(envPostgresPassword)),
		Host:   l.string(envPostgresHost) + ":" + l.string(envPostgresPort),
		Path:   "/" + l.string(envPostgresDB),
	}

	cfg := DatabaseConfig{
		DatabaseURL:       dsn.String(),
		MaxConns:          l.int32(envDBMaxConns),
		MinConns:          l.int32(envDBMinConns),
		MaxConnLifetime:   l.duration(envDBMaxConnLifetime),
		MaxConnIdleTime:   l.duration(envDBMaxConnIdleTime),
		HealthCheckPeriod: l.duration(envDBHealthCheckPeriod),
	}

	return cfg, l.err()
}
