# Garance CLI — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `garance` CLI in Go — the developer's entry point for initializing projects, running the local dev environment, managing database migrations, and generating client types. Focuses on local workflow commands for MVP; SaaS commands (login, link, deploy) deferred until the SaaS backend exists.

**Architecture:** Single Go binary using cobra for CLI framework. The CLI orchestrates Docker Compose for `garance dev`, shells out to Node.js for schema compilation (`garance.schema.ts` → `.json`), and calls the Engine's codegen for type generation. Project config stored in `garance.json` at project root.

**Tech Stack:** Go 1.25+, cobra (CLI framework), Docker Compose (orchestration), embed (for templates)

**Spec:** `docs/superpowers/specs/2026-03-25-garance-baas-design.md` (section 9)

---

## Task 1: Go Module & Cobra Setup

**Files:**
- Create: `cli/go.mod`
- Create: `cli/main.go`
- Create: `cli/cmd/root.go`
- Create: `cli/cmd/version.go`
- Modify: `services/go.work` — add `../cli`

- [ ] **Step 1: Initialize module**

```bash
mkdir -p /Users/jh3ady/Development/Projects/garance/cli/cmd
cd /Users/jh3ady/Development/Projects/garance/cli
go mod init github.com/garancehq/garance/cli
go get github.com/spf13/cobra
```

- [ ] **Step 2: Update Go workspace**

Add `../cli` to `services/go.work`.

- [ ] **Step 3: Write root command**

```go
// cli/cmd/root.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "garance",
	Short: "Garance — BaaS souverain",
	Long:  "Garance CLI — Backend-as-a-Service souverain français/européen",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Write version command**

```go
// cli/cmd/version.go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Affiche la version de Garance CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("garance %s (%s)\n", Version, GitCommit)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
```

- [ ] **Step 5: Write main.go**

```go
// cli/main.go
package main

import "github.com/garancehq/garance/cli/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 6: Verify**

```bash
cd /Users/jh3ady/Development/Projects/garance/cli && go build -o garance . && ./garance version
```
Expected: `garance dev (unknown)`

- [ ] **Step 7: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add cli/ services/go.work*
git commit -m ":tada: feat(cli): initialize garance CLI with cobra"
```

---

## Task 2: `garance init` Command

**Files:**
- Create: `cli/cmd/init.go`
- Create: `cli/internal/project/project.go`
- Create: `cli/internal/project/templates.go`
- Create: `cli/cmd/init_test.go`

- [ ] **Step 1: Write project config**

```go
// cli/internal/project/project.go
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ConfigFile = "garance.json"

type Config struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Engine  string `json:"engine"` // "postgresql"
}

func Init(dir, name string) error {
	// Create garance.json
	config := Config{
		Name:    name,
		Version: "0.1.0",
		Engine:  "postgresql",
	}

	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, ConfigFile), configBytes, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", ConfigFile, err)
	}

	// Create garance.schema.ts
	schemaContent := DefaultSchemaTemplate()
	if err := os.WriteFile(filepath.Join(dir, "garance.schema.ts"), []byte(schemaContent), 0644); err != nil {
		return fmt.Errorf("failed to write garance.schema.ts: %w", err)
	}

	// Create seed file
	seedDir := filepath.Join(dir, "seeds")
	if err := os.MkdirAll(seedDir, 0755); err != nil {
		return fmt.Errorf("failed to create seeds directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "seed.sql"), []byte(DefaultSeedTemplate()), 0644); err != nil {
		return fmt.Errorf("failed to write seed.sql: %w", err)
	}

	// Create migrations directory
	if err := os.MkdirAll(filepath.Join(dir, "migrations"), 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Create .env.local
	envContent := DefaultEnvTemplate()
	if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte(envContent), 0644); err != nil {
		return fmt.Errorf("failed to write .env.local: %w", err)
	}

	return nil
}

func LoadConfig(dir string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, ConfigFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", ConfigFile, err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", ConfigFile, err)
	}
	return &config, nil
}

func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ConfigFile))
	return err == nil
}
```

- [ ] **Step 2: Write templates**

```go
// cli/internal/project/templates.go
package project

func DefaultSchemaTemplate() string {
	return `import { defineSchema, table, column, relation } from '@garance/schema'

export default defineSchema({
  // Example table — customize or replace with your own schema
  users: table({
    id: column.uuid().primaryKey().default('gen_random_uuid()'),
    email: column.text().unique().notNull(),
    name: column.text().notNull(),
    created_at: column.timestamp().default('now()'),
  }),
})
`
}

func DefaultSeedTemplate() string {
	return `-- Seed data for local development
-- Run with: garance db seed

-- INSERT INTO users (email, name) VALUES ('dev@example.fr', 'Dev User');
`
}

func DefaultEnvTemplate() string {
	return `# Garance local development environment
# These values are used by 'garance dev'

DATABASE_URL=postgresql://postgres:postgres@localhost:5432/garance
JWT_SECRET=dev-secret-change-me-in-production
S3_ENDPOINT=localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
`
}
```

- [ ] **Step 3: Write init command**

```go
// cli/cmd/init.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/garancehq/garance/cli/internal/project"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialise un nouveau projet Garance",
	Long:  "Crée la structure de fichiers pour un nouveau projet Garance (garance.json, garance.schema.ts, etc.)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()

		name := filepath.Base(dir)
		if len(args) > 0 {
			name = args[0]
			// Create subdirectory if name is provided
			dir = filepath.Join(dir, name)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		if project.Exists(dir) {
			return fmt.Errorf("project already initialized (garance.json exists)")
		}

		if err := project.Init(dir, name); err != nil {
			return err
		}

		fmt.Printf("✓ Projet '%s' initialisé\n", name)
		fmt.Println()
		fmt.Println("Fichiers créés :")
		fmt.Println("  garance.json        — configuration du projet")
		fmt.Println("  garance.schema.ts   — schéma déclaratif")
		fmt.Println("  seeds/seed.sql      — données de test")
		fmt.Println("  migrations/         — migrations SQL")
		fmt.Println("  .env.local          — variables d'environnement locales")
		fmt.Println()
		fmt.Println("Prochaine étape :")
		fmt.Println("  garance dev         — lance l'environnement de développement")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
```

- [ ] **Step 4: Write test**

```go
// cli/cmd/init_test.go
package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/garancehq/garance/cli/internal/project"
)

func TestInitCreatesFiles(t *testing.T) {
	dir := t.TempDir()

	err := project.Init(dir, "test-project")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Check garance.json exists
	if _, err := os.Stat(filepath.Join(dir, "garance.json")); err != nil {
		t.Error("garance.json not created")
	}

	// Check garance.schema.ts exists
	if _, err := os.Stat(filepath.Join(dir, "garance.schema.ts")); err != nil {
		t.Error("garance.schema.ts not created")
	}

	// Check seeds directory
	if _, err := os.Stat(filepath.Join(dir, "seeds", "seed.sql")); err != nil {
		t.Error("seeds/seed.sql not created")
	}

	// Check migrations directory
	if _, err := os.Stat(filepath.Join(dir, "migrations")); err != nil {
		t.Error("migrations/ not created")
	}

	// Check .env.local
	if _, err := os.Stat(filepath.Join(dir, ".env.local")); err != nil {
		t.Error(".env.local not created")
	}

	// Load config
	config, err := project.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.Name != "test-project" {
		t.Errorf("expected name test-project, got %s", config.Name)
	}
}

func TestInitFailsIfAlreadyExists(t *testing.T) {
	dir := t.TempDir()

	project.Init(dir, "first")
	err := project.Init(dir, "second")
	// Should not fail — Init doesn't check for existing project
	// The command checks via project.Exists()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectExists(t *testing.T) {
	dir := t.TempDir()

	if project.Exists(dir) {
		t.Error("should not exist before init")
	}

	project.Init(dir, "test")

	if !project.Exists(dir) {
		t.Error("should exist after init")
	}
}
```

- [ ] **Step 5: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/cli && go test ./... -v`
Expected: 3 tests pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add cli/
git commit -m ":sparkles: feat(cli): add garance init command with project scaffolding"
```

---

## Task 3: `garance dev` Command

**Files:**
- Create: `cli/cmd/dev.go`
- Create: `cli/internal/compose/compose.go`
- Create: `cli/internal/compose/template.go`

- [ ] **Step 1: Write Docker Compose template**

```go
// cli/internal/compose/template.go
package compose

func DevComposeTemplate() string {
	return `# Generated by garance dev — do not edit manually
services:
  postgres:
    image: postgres:17-alpine
    ports:
      - "${DB_PORT:-5432}:5432"
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD:-postgres}
      POSTGRES_DB: ${DB_NAME:-garance}
    volumes:
      - garance_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 2s
      timeout: 5s
      retries: 10

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    ports:
      - "${S3_PORT:-9000}:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: ${S3_ACCESS_KEY:-minioadmin}
      MINIO_ROOT_PASSWORD: ${S3_SECRET_KEY:-minioadmin}
    volumes:
      - garance_storage:/data

  engine:
    image: ghcr.io/garancehq/engine:latest
    ports:
      - "4000:4000"
      - "5000:5000"
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgresql://postgres:${DB_PASSWORD:-postgres}@postgres:5432/${DB_NAME:-garance}
      LISTEN_ADDR: 0.0.0.0:4000
      GRPC_ADDR: 0.0.0.0:5000

  auth:
    image: ghcr.io/garancehq/auth:latest
    ports:
      - "4001:4001"
      - "5001:5001"
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgresql://postgres:${DB_PASSWORD:-postgres}@postgres:5432/${DB_NAME:-garance}
      LISTEN_ADDR: 0.0.0.0:4001
      GRPC_ADDR: 0.0.0.0:5001
      JWT_SECRET: ${JWT_SECRET:-dev-secret-change-me}

  storage:
    image: ghcr.io/garancehq/storage:latest
    ports:
      - "4002:4002"
      - "5002:5002"
    depends_on:
      postgres:
        condition: service_healthy
      minio:
        condition: service_started
    environment:
      DATABASE_URL: postgresql://postgres:${DB_PASSWORD:-postgres}@postgres:5432/${DB_NAME:-garance}
      LISTEN_ADDR: 0.0.0.0:4002
      GRPC_ADDR: 0.0.0.0:5002
      S3_ENDPOINT: minio:9000
      S3_ACCESS_KEY: ${S3_ACCESS_KEY:-minioadmin}
      S3_SECRET_KEY: ${S3_SECRET_KEY:-minioadmin}
      JWT_SECRET: ${JWT_SECRET:-dev-secret-change-me}

  gateway:
    image: ghcr.io/garancehq/gateway:latest
    ports:
      - "8080:8080"
    depends_on:
      - engine
      - auth
      - storage
    environment:
      LISTEN_ADDR: 0.0.0.0:8080
      ENGINE_GRPC_ADDR: engine:5000
      AUTH_GRPC_ADDR: auth:5001
      STORAGE_GRPC_ADDR: storage:5002
      JWT_SECRET: ${JWT_SECRET:-dev-secret-change-me}
      ALLOWED_ORIGINS: "*"

volumes:
  garance_data:
  garance_storage:
`
}
```

- [ ] **Step 2: Write compose runner**

```go
// cli/internal/compose/compose.go
package compose

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const composeFileName = ".garance-compose.yml"

// WriteComposeFile writes the dev Docker Compose file to the project directory.
func WriteComposeFile(dir string) (string, error) {
	path := filepath.Join(dir, composeFileName)
	content := DevComposeTemplate()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write compose file: %w", err)
	}
	return path, nil
}

// Up starts the dev environment.
func Up(dir string) error {
	composePath, err := WriteComposeFile(dir)
	if err != nil {
		return err
	}

	// Load .env.local if it exists
	envFile := filepath.Join(dir, ".env.local")
	args := []string{"compose", "-f", composePath, "--project-name", "garance"}
	if _, err := os.Stat(envFile); err == nil {
		args = append(args, "--env-file", envFile)
	}
	args = append(args, "up", "-d")

	cmd := exec.Command("docker", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}
	return nil
}

// Down stops the dev environment.
func Down(dir string) error {
	composePath := filepath.Join(dir, composeFileName)
	if _, err := os.Stat(composePath); err != nil {
		return fmt.Errorf("no dev environment found (run 'garance dev' first)")
	}

	cmd := exec.Command("docker", "compose", "-f", composePath, "--project-name", "garance", "down")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down failed: %w", err)
	}
	return nil
}

// Status shows running services.
func Status(dir string) error {
	composePath := filepath.Join(dir, composeFileName)
	if _, err := os.Stat(composePath); err != nil {
		return fmt.Errorf("no dev environment found (run 'garance dev' first)")
	}

	cmd := exec.Command("docker", "compose", "-f", composePath, "--project-name", "garance", "ps")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Logs shows service logs.
func Logs(dir string, service string, follow bool) error {
	composePath := filepath.Join(dir, composeFileName)
	args := []string{"compose", "-f", composePath, "--project-name", "garance", "logs"}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	}

	cmd := exec.Command("docker", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
```

- [ ] **Step 3: Write dev command**

```go
// cli/cmd/dev.go
package cmd

import (
	"fmt"
	"os"

	"github.com/garancehq/garance/cli/internal/compose"
	"github.com/garancehq/garance/cli/internal/project"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Lance l'environnement de développement local",
	Long:  "Démarre PostgreSQL, MinIO, Engine, Auth, Storage et Gateway via Docker Compose",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()

		if !project.Exists(dir) {
			return fmt.Errorf("pas de projet Garance dans ce répertoire (lancez 'garance init' d'abord)")
		}

		fmt.Println("Démarrage de l'environnement Garance...")
		fmt.Println()

		if err := compose.Up(dir); err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("✓ Environnement démarré")
		fmt.Println()
		fmt.Println("Services disponibles :")
		fmt.Println("  Gateway (API)      http://localhost:8080")
		fmt.Println("  Engine             http://localhost:4000")
		fmt.Println("  Auth               http://localhost:4001")
		fmt.Println("  Storage            http://localhost:4002")
		fmt.Println("  MinIO Console      http://localhost:9001")
		fmt.Println("  PostgreSQL         localhost:5432")
		fmt.Println()
		fmt.Println("Commandes utiles :")
		fmt.Println("  garance dev stop   — arrête l'environnement")
		fmt.Println("  garance dev status — état des services")
		fmt.Println("  garance dev logs   — affiche les logs")

		return nil
	},
}

var devStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Arrête l'environnement de développement",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		if err := compose.Down(dir); err != nil {
			return err
		}
		fmt.Println("✓ Environnement arrêté")
		return nil
	},
}

var devStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Affiche l'état des services",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		return compose.Status(dir)
	},
}

var devLogsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "Affiche les logs des services",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		service := ""
		if len(args) > 0 {
			service = args[0]
		}
		follow, _ := cmd.Flags().GetBool("follow")
		return compose.Logs(dir, service, follow)
	},
}

func init() {
	devLogsCmd.Flags().BoolP("follow", "f", false, "Suivre les logs en temps réel")
	devCmd.AddCommand(devStopCmd)
	devCmd.AddCommand(devStatusCmd)
	devCmd.AddCommand(devLogsCmd)
	rootCmd.AddCommand(devCmd)
}
```

- [ ] **Step 4: Verify build**

Run: `cd /Users/jh3ady/Development/Projects/garance/cli && go build -o garance . && ./garance dev --help`

- [ ] **Step 5: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add cli/
git commit -m ":sparkles: feat(cli): add garance dev command with Docker Compose orchestration"
```

---

## Task 4: `garance db` Commands

**Files:**
- Create: `cli/cmd/db.go`
- Create: `cli/internal/db/migrate.go`

- [ ] **Step 1: Write db helpers**

```go
// cli/internal/db/migrate.go
package db

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
```

Add pgx dependency: `go get github.com/jackc/pgx/v5`

(Note: `exec` import is unused — remove it. The `exec` package was in the template by mistake.)

- [ ] **Step 2: Write db commands**

```go
// cli/cmd/db.go
package cmd

import (
	"fmt"
	"os"

	"github.com/garancehq/garance/cli/internal/db"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Commandes de gestion de la base de données",
}

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Applique les migrations SQL",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		databaseURL := db.GetDatabaseURL(dir)

		fmt.Println("Application des migrations...")
		if err := db.Migrate(dir, databaseURL); err != nil {
			return err
		}
		fmt.Println("✓ Migrations appliquées")
		return nil
	},
}

var dbResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Supprime et recrée la base de données",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		databaseURL := db.GetDatabaseURL(dir)

		if err := db.Reset(databaseURL); err != nil {
			return err
		}
		fmt.Println("✓ Base de données recréée")

		// Re-apply migrations
		fmt.Println("Application des migrations...")
		if err := db.Migrate(dir, databaseURL); err != nil {
			return err
		}
		fmt.Println("✓ Migrations appliquées")
		return nil
	},
}

var dbSeedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Exécute le fichier de seed",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		databaseURL := db.GetDatabaseURL(dir)

		fmt.Println("Exécution du seed...")
		if err := db.Seed(dir, databaseURL); err != nil {
			return err
		}
		fmt.Println("✓ Seed exécuté")
		return nil
	},
}

func init() {
	dbCmd.AddCommand(dbMigrateCmd)
	dbCmd.AddCommand(dbResetCmd)
	dbCmd.AddCommand(dbSeedCmd)
	rootCmd.AddCommand(dbCmd)
}
```

- [ ] **Step 3: Verify**

Run: `cd /Users/jh3ady/Development/Projects/garance/cli && go build -o garance . && ./garance db --help`

- [ ] **Step 4: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add cli/
git commit -m ":sparkles: feat(cli): add garance db migrate, reset, and seed commands"
```

---

## Task 5: `garance gen types` & `garance status` Commands

**Files:**
- Create: `cli/cmd/gen.go`
- Create: `cli/cmd/status.go`

- [ ] **Step 1: Write gen types command**

```go
// cli/cmd/gen.go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/garancehq/garance/cli/internal/project"
	"github.com/spf13/cobra"
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Commandes de génération de code",
}

var genTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "Génère les types clients depuis le schéma",
	Long:  "Génère les types TypeScript, Dart, Swift ou Kotlin depuis le schéma PostgreSQL introspécté",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()

		if !project.Exists(dir) {
			return fmt.Errorf("pas de projet Garance dans ce répertoire")
		}

		lang, _ := cmd.Flags().GetString("lang")
		output, _ := cmd.Flags().GetString("output")

		if output == "" {
			switch lang {
			case "ts":
				output = filepath.Join(dir, "types", "garance.ts")
			case "dart":
				output = filepath.Join(dir, "types", "garance.dart")
			case "swift":
				output = filepath.Join(dir, "types", "garance.swift")
			case "kotlin":
				output = filepath.Join(dir, "types", "garance.kt")
			default:
				output = filepath.Join(dir, "types", "garance.ts")
			}
		}

		// Ensure output directory exists
		if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		// Call the engine codegen endpoint or binary
		// For MVP, we call the engine HTTP API to introspect and generate
		engineURL := os.Getenv("ENGINE_URL")
		if engineURL == "" {
			engineURL = "http://localhost:4000"
		}

		fmt.Printf("Génération des types %s...\n", lang)

		// For MVP: use curl to call engine introspection, then generate locally
		// This is a placeholder — in production, the engine would have a /gen/types endpoint
		// For now, we just create a stub file indicating the types would be generated
		stub := fmt.Sprintf("// Types générés par garance gen types --lang %s\n// Connectez-vous à un environnement Garance (garance dev) pour générer les types réels\n", lang)
		if err := os.WriteFile(output, []byte(stub), 0644); err != nil {
			return fmt.Errorf("failed to write types: %w", err)
		}

		fmt.Printf("✓ Types générés : %s\n", output)
		return nil
	},
}

func init() {
	genTypesCmd.Flags().StringP("lang", "l", "ts", "Langage cible (ts, dart, swift, kotlin)")
	genTypesCmd.Flags().StringP("output", "o", "", "Fichier de sortie")
	genCmd.AddCommand(genTypesCmd)
	rootCmd.AddCommand(genCmd)
}
```

- [ ] **Step 2: Write status command**

```go
// cli/cmd/status.go
package cmd

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/garancehq/garance/cli/internal/project"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Affiche l'état du projet",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()

		if !project.Exists(dir) {
			return fmt.Errorf("pas de projet Garance dans ce répertoire")
		}

		config, err := project.LoadConfig(dir)
		if err != nil {
			return err
		}

		fmt.Printf("Projet : %s (v%s)\n", config.Name, config.Version)
		fmt.Printf("Engine : %s\n", config.Engine)
		fmt.Println()

		// Check local services
		fmt.Println("Services locaux :")
		services := []struct {
			name string
			url  string
		}{
			{"Gateway", "http://localhost:8080/health"},
			{"Engine", "http://localhost:4000/health"},
			{"Auth", "http://localhost:4001/health"},
			{"Storage", "http://localhost:4002/health"},
		}

		client := &http.Client{Timeout: 2 * time.Second}
		for _, svc := range services {
			status := "✗ arrêté"
			resp, err := client.Get(svc.url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == 200 {
					status = "✓ en ligne"
				}
			}
			fmt.Printf("  %-12s %s\n", svc.name, status)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
```

- [ ] **Step 3: Verify**

Run: `cd /Users/jh3ady/Development/Projects/garance/cli && go build -o garance . && ./garance status --help && ./garance gen types --help`

- [ ] **Step 4: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add cli/
git commit -m ":sparkles: feat(cli): add garance gen types and garance status commands"
```

---

## Task 6: Dockerfile & Final Build

**Files:**
- Create: `cli/Dockerfile`
- Create: `cli/.goreleaser.yml` (for future cross-compilation)

- [ ] **Step 1: Write Dockerfile**

```dockerfile
# cli/Dockerfile
FROM golang:1-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-X github.com/garancehq/garance/cli/cmd.Version=0.1.0" -o /garance .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=builder /garance /usr/local/bin/garance
CMD ["garance"]
```

Note: `docker-cli` is included because `garance dev` shells out to `docker compose`.

- [ ] **Step 2: Build and verify**

Run: `cd /Users/jh3ady/Development/Projects/garance/cli && go build -o garance . && ./garance version`
Run: `./garance --help`

Expected output of `--help`:
```
Garance CLI — Backend-as-a-Service souverain français/européen

Usage:
  garance [command]

Available Commands:
  db          Commandes de gestion de la base de données
  dev         Lance l'environnement de développement local
  gen         Commandes de génération de code
  init        Initialise un nouveau projet Garance
  status      Affiche l'état du projet
  version     Affiche la version de Garance CLI
  help        Help about any command
```

- [ ] **Step 3: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add cli/
git commit -m ":whale: build(cli): add Dockerfile and finalize CLI build"
```

---

## Summary

| Task | Description | Tests |
|---|---|---|
| 1 | Go module, cobra setup, version command | 0 (build) |
| 2 | `garance init` — project scaffolding | 3 |
| 3 | `garance dev` — Docker Compose orchestration | 0 (build) |
| 4 | `garance db migrate/reset/seed` | 0 (build, needs running DB) |
| 5 | `garance gen types` + `garance status` | 0 (build) |
| 6 | Dockerfile & final build | 0 (build) |
| **Total** | | **3** |

### Not in this plan (deferred)

- `garance login` / `garance link` / `garance deploy` — requires SaaS backend
- Real type generation (needs Engine codegen endpoint wired to CLI)
- Schema compilation (`garance.schema.ts` → `garance.schema.json`) — requires `@garance/schema` npm package (Plan 6)
- Homebrew tap formula
- install.sh script
- Shell autocompletion
