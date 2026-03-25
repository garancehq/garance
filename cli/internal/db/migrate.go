// cli/internal/db/migrate.go
package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
)

// Migrate applies pending SQL migration files from the migrations/ directory.
func Migrate(dir, databaseURL string) error {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	migrationsDir := filepath.Join(dir, "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("Aucune migration à appliquer")
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".sql" {
			continue
		}

		sql, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", name, err)
		}

		fmt.Printf("Applying %s...\n", name)
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("failed to apply %s: %w", name, err)
		}
	}

	return nil
}

// Reset drops and recreates the database.
func Reset(databaseURL string) error {
	ctx := context.Background()

	// Connect to 'postgres' database to drop/recreate target
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("invalid database URL: %w", err)
	}
	dbName := cfg.Database
	cfg.Database = "postgres"

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close(ctx)

	// Terminate existing connections
	conn.Exec(ctx, fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()", dbName))

	fmt.Printf("Dropping database %s...\n", dbName)
	if _, err := conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS \"%s\"", dbName)); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	fmt.Printf("Creating database %s...\n", dbName)
	if _, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE \"%s\"", dbName)); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	return nil
}

// Seed executes the seed file.
func Seed(dir, databaseURL string) error {
	seedFile := filepath.Join(dir, "seeds", "seed.sql")
	if _, err := os.Stat(seedFile); err != nil {
		return fmt.Errorf("seed file not found: %s", seedFile)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close(ctx)

	sql, err := os.ReadFile(seedFile)
	if err != nil {
		return fmt.Errorf("failed to read seed file: %w", err)
	}

	if _, err := conn.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("failed to execute seed: %w", err)
	}

	return nil
}

// GetDatabaseURL reads DATABASE_URL from .env.local or environment.
func GetDatabaseURL(dir string) string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	// Try .env.local
	envFile := filepath.Join(dir, ".env.local")
	data, err := os.ReadFile(envFile)
	if err == nil {
		for _, line := range splitLines(string(data)) {
			if len(line) > 13 && line[:13] == "DATABASE_URL=" {
				return line[13:]
			}
		}
	}
	return "postgresql://postgres:postgres@localhost:5432/garance"
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
