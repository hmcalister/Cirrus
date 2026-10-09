// Package postgres constructs the database connection pool used by the
// application.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/hmcalister/LiteralCloudService/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a connection pool against the given libpq URL and verifies it with a ping before returning.
//
// A pool is used rather than a single pgx.Conn because a pgx.Conn is not safe for concurrent use.
//
// The caller is responsible for calling Close on the returned pool.
func Connect(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database url: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}
