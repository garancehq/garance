package store

import (
	"context"
	"fmt"
	"os"
	"strings"

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

// RunMigrationsFromDir reads and applies all .sql migrations from the given directory in lexicographic order.
func (db *DB) RunMigrationsFromDir(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		sql, err := os.ReadFile(dir + "/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", entry.Name(), err)
		}
		if _, err := db.Pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("failed to apply %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// RunMigrationsFromSQL applies migration SQL directly (for testing).
func (db *DB) RunMigrationsFromSQL(ctx context.Context, sql string) error {
	_, err := db.Pool.Exec(ctx, sql)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}
