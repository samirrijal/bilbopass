package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps pgxpool.Pool and provides a shared connection pool.
type DB struct {
	Pool *pgxpool.Pool
}

// New creates a new DB connection pool.
func New(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	cfg.MaxConns = 50

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close releases pool resources.
func (db *DB) Close() {
	db.Pool.Close()
}
