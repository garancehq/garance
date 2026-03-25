package store

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func NewDB(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

func (db *DB) RunMigrationsFromDir(ctx context.Context, dir string) error {
	sql, err := os.ReadFile(dir + "/001_initial.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}
	_, err = db.Pool.Exec(ctx, string(sql))
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}
