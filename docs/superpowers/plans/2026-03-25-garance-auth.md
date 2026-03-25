# Garance Auth Service — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Garance Auth Service in Go — handles user signup/signin, JWT token management, magic links, OAuth2 providers, and email verification. Exposes an HTTP API (gRPC will be added when the Gateway is built in Plan 4).

**Architecture:** Single Go module with clean package separation: `handler` (HTTP routes), `service` (business logic), `store` (PostgreSQL persistence), `token` (JWT + refresh), `mail` (email sending), `oauth` (OAuth2 providers). All auth data lives in the `garance_auth` PostgreSQL schema, isolated from user data.

**Tech Stack:** Go 1.23+, Chi router (HTTP), pgx v5 (PostgreSQL), golang-jwt v5 (JWT), argon2id (password hashing), gomail (SMTP), testcontainers-go (integration tests)

**Spec:** `docs/superpowers/specs/2026-03-25-garance-baas-design.md` (sections 6, 13)

---

## Task 1: Go Module & Project Structure

**Files:**
- Create: `services/auth/go.mod`
- Create: `services/auth/go.sum` (generated)
- Create: `services/auth/main.go`
- Create: `services/auth/internal/config/config.go`
- Create: `services/go.work`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/jh3ady/Development/Projects/garance/services/auth
go mod init github.com/garancehq/garance/services/auth
```

- [ ] **Step 2: Create Go workspace**

```go
// services/go.work
go 1.23

use (
    ./auth
)
```

- [ ] **Step 3: Write config**

```go
// services/auth/internal/config/config.go
package config

import "os"

type Config struct {
	DatabaseURL string
	ListenAddr  string
	JWTSecret   string
	SMTPHost    string
	SMTPPort    string
	SMTPUser    string
	SMTPPass    string
	SMTPFrom    string
	BaseURL     string // e.g. https://mon-projet.garance.io
}

func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/postgres"),
		ListenAddr:  getEnv("LISTEN_ADDR", "0.0.0.0:4001"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-me"),
		SMTPHost:    getEnv("SMTP_HOST", "localhost"),
		SMTPPort:    getEnv("SMTP_PORT", "1025"),
		SMTPUser:    getEnv("SMTP_USER", ""),
		SMTPPass:    getEnv("SMTP_PASS", ""),
		SMTPFrom:    getEnv("SMTP_FROM", "noreply@garance.io"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:4001"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
```

- [ ] **Step 4: Write main.go stub**

```go
// services/auth/main.go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/garancehq/garance/services/auth/internal/config"
)

func main() {
	cfg := config.Load()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Printf("garance auth service listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatal(err)
	}
	_ = fmt.Sprintf("base url: %s", cfg.BaseURL)
}
```

- [ ] **Step 5: Add dependencies**

```bash
cd /Users/jh3ady/Development/Projects/garance/services/auth
go get github.com/jackc/pgx/v5
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto
go get github.com/google/uuid
```

- [ ] **Step 6: Verify it compiles and runs**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/auth && go build ./...`
Expected: Compiles without errors.

- [ ] **Step 7: Commit**

```bash
git add services/
git commit -m ":tada: feat(auth): initialize Go module with config and health endpoint"
```

---

## Task 2: Database Schema & Store

**Files:**
- Create: `services/auth/migrations/001_initial.sql`
- Create: `services/auth/internal/store/db.go`
- Create: `services/auth/internal/store/user.go`
- Create: `services/auth/internal/store/session.go`
- Create: `services/auth/internal/store/identity.go`
- Create: `services/auth/internal/store/verification.go`
- Create: `services/auth/internal/store/store_test.go`

- [ ] **Step 1: Write migration SQL**

```sql
-- services/auth/migrations/001_initial.sql
CREATE SCHEMA IF NOT EXISTS garance_auth;

CREATE TABLE garance_auth.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    encrypted_password TEXT, -- NULL for OAuth-only / magic link users
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    role TEXT NOT NULL DEFAULT 'user',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    banned_at TIMESTAMPTZ
);

CREATE TABLE garance_auth.sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES garance_auth.users(id) ON DELETE CASCADE,
    refresh_token TEXT UNIQUE NOT NULL,
    user_agent TEXT,
    ip_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user_id ON garance_auth.sessions(user_id);
CREATE INDEX idx_sessions_refresh_token ON garance_auth.sessions(refresh_token) WHERE revoked_at IS NULL;

CREATE TABLE garance_auth.identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES garance_auth.users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL, -- 'google', 'github', 'gitlab'
    provider_user_id TEXT NOT NULL,
    provider_data JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX idx_identities_user_id ON garance_auth.identities(user_id);

CREATE TABLE garance_auth.verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES garance_auth.users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL, -- 'email_verification', 'password_reset', 'magic_link'
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);

CREATE INDEX idx_verification_tokens_token ON garance_auth.verification_tokens(token) WHERE used_at IS NULL;
```

- [ ] **Step 2: Write database connection**

```go
// services/auth/internal/store/db.go
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

// RunMigrationsFromDir reads and applies SQL migrations from the given directory.
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

// RunMigrationsFromSQL applies migration SQL directly (for testing).
func (db *DB) RunMigrationsFromSQL(ctx context.Context, sql string) error {
	_, err := db.Pool.Exec(ctx, sql)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Write user store**

```go
// services/auth/internal/store/user.go
package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrEmailAlreadyTaken = errors.New("email already taken")
)

type User struct {
	ID                uuid.UUID  `json:"id"`
	Email             string     `json:"email"`
	EncryptedPassword *string    `json:"-"`
	EmailVerified     bool       `json:"email_verified"`
	Role              string     `json:"role"`
	Metadata          []byte     `json:"metadata"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	BannedAt          *time.Time `json:"banned_at,omitempty"`
}

func (db *DB) CreateUser(ctx context.Context, email string, encryptedPassword *string) (*User, error) {
	var user User
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO garance_auth.users (email, encrypted_password)
		 VALUES ($1, $2)
		 RETURNING id, email, encrypted_password, email_verified, role, metadata, created_at, updated_at, banned_at`,
		email, encryptedPassword,
	).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &user.EmailVerified,
		&user.Role, &user.Metadata, &user.CreatedAt, &user.UpdatedAt, &user.BannedAt)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, ErrEmailAlreadyTaken
		}
		return nil, err
	}
	return &user, nil
}

func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	err := db.Pool.QueryRow(ctx,
		`SELECT id, email, encrypted_password, email_verified, role, metadata, created_at, updated_at, banned_at
		 FROM garance_auth.users WHERE id = $1`, id,
	).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &user.EmailVerified,
		&user.Role, &user.Metadata, &user.CreatedAt, &user.UpdatedAt, &user.BannedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

func (db *DB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := db.Pool.QueryRow(ctx,
		`SELECT id, email, encrypted_password, email_verified, role, metadata, created_at, updated_at, banned_at
		 FROM garance_auth.users WHERE email = $1`, email,
	).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &user.EmailVerified,
		&user.Role, &user.Metadata, &user.CreatedAt, &user.UpdatedAt, &user.BannedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

func (db *DB) VerifyUserEmail(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE garance_auth.users SET email_verified = TRUE, updated_at = now() WHERE id = $1`, id)
	return err
}

func (db *DB) UpdateUserPassword(ctx context.Context, id uuid.UUID, encryptedPassword string) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE garance_auth.users SET encrypted_password = $1, updated_at = now() WHERE id = $2`,
		encryptedPassword, id)
	return err
}

func (db *DB) DeleteUser(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM garance_auth.users WHERE id = $1`, id)
	return err
}

func isDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}
	return false
}
```

- [ ] **Step 4: Write session store**

```go
// services/auth/internal/store/session.go
package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrSessionNotFound = errors.New("session not found")

type Session struct {
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	RefreshToken string     `json:"-"`
	UserAgent    *string    `json:"user_agent,omitempty"`
	IPAddress    *string    `json:"ip_address,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

func (db *DB) CreateSession(ctx context.Context, userID uuid.UUID, refreshToken, userAgent, ipAddress string) (*Session, error) {
	var session Session
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO garance_auth.sessions (user_id, refresh_token, user_agent, ip_address, expires_at)
		 VALUES ($1, $2, $3, $4, now() + interval '30 days')
		 RETURNING id, user_id, refresh_token, user_agent, ip_address, created_at, expires_at, revoked_at`,
		userID, refreshToken, nilIfEmpty(userAgent), nilIfEmpty(ipAddress),
	).Scan(&session.ID, &session.UserID, &session.RefreshToken, &session.UserAgent,
		&session.IPAddress, &session.CreatedAt, &session.ExpiresAt, &session.RevokedAt)
	return &session, err
}

func (db *DB) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error) {
	var session Session
	err := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, refresh_token, user_agent, ip_address, created_at, expires_at, revoked_at
		 FROM garance_auth.sessions
		 WHERE refresh_token = $1 AND revoked_at IS NULL AND expires_at > now()`,
		refreshToken,
	).Scan(&session.ID, &session.UserID, &session.RefreshToken, &session.UserAgent,
		&session.IPAddress, &session.CreatedAt, &session.ExpiresAt, &session.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	return &session, err
}

func (db *DB) RevokeSession(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE garance_auth.sessions SET revoked_at = now() WHERE id = $1`, id)
	return err
}

func (db *DB) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE garance_auth.sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```

- [ ] **Step 5: Write identity store (for OAuth)**

```go
// services/auth/internal/store/identity.go
package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrIdentityNotFound = errors.New("identity not found")

type Identity struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"provider_user_id"`
	ProviderData   []byte    `json:"provider_data"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (db *DB) CreateIdentity(ctx context.Context, userID uuid.UUID, provider, providerUserID string, providerData []byte) (*Identity, error) {
	var identity Identity
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO garance_auth.identities (user_id, provider, provider_user_id, provider_data)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, provider, provider_user_id, provider_data, created_at, updated_at`,
		userID, provider, providerUserID, providerData,
	).Scan(&identity.ID, &identity.UserID, &identity.Provider, &identity.ProviderUserID,
		&identity.ProviderData, &identity.CreatedAt, &identity.UpdatedAt)
	return &identity, err
}

func (db *DB) GetIdentityByProvider(ctx context.Context, provider, providerUserID string) (*Identity, error) {
	var identity Identity
	err := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, provider, provider_user_id, provider_data, created_at, updated_at
		 FROM garance_auth.identities WHERE provider = $1 AND provider_user_id = $2`,
		provider, providerUserID,
	).Scan(&identity.ID, &identity.UserID, &identity.Provider, &identity.ProviderUserID,
		&identity.ProviderData, &identity.CreatedAt, &identity.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrIdentityNotFound
	}
	return &identity, err
}
```

- [ ] **Step 6: Write verification token store**

```go
// services/auth/internal/store/verification.go
package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrTokenNotFound = errors.New("verification token not found")

type VerificationToken struct {
	ID        uuid.UUID  `json:"id"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	Email     string     `json:"email"`
	Token     string     `json:"-"`
	Type      string     `json:"type"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}

func (db *DB) CreateVerificationToken(ctx context.Context, userID *uuid.UUID, email, token, tokenType string, ttl time.Duration) (*VerificationToken, error) {
	var vt VerificationToken
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO garance_auth.verification_tokens (user_id, email, token, type, expires_at)
		 VALUES ($1, $2, $3, $4, now() + $5::interval)
		 RETURNING id, user_id, email, token, type, created_at, expires_at, used_at`,
		userID, email, token, tokenType, ttl.String(),
	).Scan(&vt.ID, &vt.UserID, &vt.Email, &vt.Token, &vt.Type,
		&vt.CreatedAt, &vt.ExpiresAt, &vt.UsedAt)
	return &vt, err
}

func (db *DB) GetVerificationToken(ctx context.Context, token string) (*VerificationToken, error) {
	var vt VerificationToken
	err := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, email, token, type, created_at, expires_at, used_at
		 FROM garance_auth.verification_tokens
		 WHERE token = $1 AND used_at IS NULL AND expires_at > now()`,
		token,
	).Scan(&vt.ID, &vt.UserID, &vt.Email, &vt.Token, &vt.Type,
		&vt.CreatedAt, &vt.ExpiresAt, &vt.UsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTokenNotFound
	}
	return &vt, err
}

func (db *DB) MarkTokenUsed(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE garance_auth.verification_tokens SET used_at = now() WHERE id = $1`, id)
	return err
}
```

- [ ] **Step 7: Write integration test**

```go
// services/auth/internal/store/store_test.go
package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/garancehq/garance/services/auth/internal/store"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupDB(t *testing.T) *store.DB {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() { pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := store.NewDB(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Use RunMigrationsFromDir with path relative to the test file location.
	// Tests run from the package directory, so we go up to the auth service root.
	if err := db.RunMigrationsFromDir(ctx, "../../migrations"); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return db
}

func TestCreateAndGetUser(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	password := "hashed_password"
	user, err := db.CreateUser(ctx, "alice@example.fr", &password)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Email != "alice@example.fr" {
		t.Errorf("expected email alice@example.fr, got %s", user.Email)
	}
	if user.EmailVerified {
		t.Error("expected email_verified to be false")
	}

	found, err := db.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if found.Email != "alice@example.fr" {
		t.Errorf("expected email alice@example.fr, got %s", found.Email)
	}

	found2, err := db.GetUserByEmail(ctx, "alice@example.fr")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if found2.ID != user.ID {
		t.Error("user IDs don't match")
	}
}

func TestDuplicateEmail(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	password := "hashed"
	_, err := db.CreateUser(ctx, "dup@example.fr", &password)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateUser(ctx, "dup@example.fr", &password)
	if err != store.ErrEmailAlreadyTaken {
		t.Errorf("expected ErrEmailAlreadyTaken, got %v", err)
	}
}

func TestVerifyEmail(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	user, _ := db.CreateUser(ctx, "verify@example.fr", nil)
	if user.EmailVerified {
		t.Fatal("should not be verified initially")
	}

	db.VerifyUserEmail(ctx, user.ID)
	found, _ := db.GetUserByID(ctx, user.ID)
	if !found.EmailVerified {
		t.Error("email should be verified")
	}
}

func TestSessionCRUD(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	user, _ := db.CreateUser(ctx, "session@example.fr", nil)
	token := uuid.New().String()

	session, err := db.CreateSession(ctx, user.ID, token, "TestAgent", "127.0.0.1")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	found, err := db.GetSessionByRefreshToken(ctx, token)
	if err != nil {
		t.Fatalf("GetSessionByRefreshToken: %v", err)
	}
	if found.ID != session.ID {
		t.Error("session IDs don't match")
	}

	db.RevokeSession(ctx, session.ID)
	_, err = db.GetSessionByRefreshToken(ctx, token)
	if err != store.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after revoke, got %v", err)
	}
}

func TestVerificationToken(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	user, _ := db.CreateUser(ctx, "token@example.fr", nil)
	tokenStr := uuid.New().String()

	vt, err := db.CreateVerificationToken(ctx, &user.ID, user.Email, tokenStr, "email_verification", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateVerificationToken: %v", err)
	}
	if vt.Type != "email_verification" {
		t.Errorf("expected type email_verification, got %s", vt.Type)
	}

	found, err := db.GetVerificationToken(ctx, tokenStr)
	if err != nil {
		t.Fatalf("GetVerificationToken: %v", err)
	}
	if found.Email != user.Email {
		t.Error("emails don't match")
	}

	db.MarkTokenUsed(ctx, vt.ID)
	_, err = db.GetVerificationToken(ctx, tokenStr)
	if err != store.ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound after marking used, got %v", err)
	}
}

func TestIdentity(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	user, _ := db.CreateUser(ctx, "oauth@example.fr", nil)
	providerData := []byte(`{"name":"Alice","avatar":"https://example.com/alice.jpg"}`)

	identity, err := db.CreateIdentity(ctx, user.ID, "google", "google-123", providerData)
	if err != nil {
		t.Fatalf("CreateIdentity: %v", err)
	}
	if identity.Provider != "google" {
		t.Errorf("expected provider google, got %s", identity.Provider)
	}

	found, err := db.GetIdentityByProvider(ctx, "google", "google-123")
	if err != nil {
		t.Fatalf("GetIdentityByProvider: %v", err)
	}
	if found.UserID != user.ID {
		t.Error("user IDs don't match")
	}

	_, err = db.GetIdentityByProvider(ctx, "github", "nonexistent")
	if err != store.ErrIdentityNotFound {
		t.Errorf("expected ErrIdentityNotFound, got %v", err)
	}
}
```

Add test dependency:
```bash
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
```

- [ ] **Step 8: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/auth && go test ./internal/store/ -v -count=1`
Expected: 6 tests pass. Requires Docker.

- [ ] **Step 9: Commit**

```bash
git add services/
git commit -m ":sparkles: feat(auth): add database schema, migrations, and store layer"
```

---

## Task 3: Password Hashing & JWT Tokens

**Files:**
- Create: `services/auth/internal/crypto/password.go`
- Create: `services/auth/internal/crypto/password_test.go`
- Create: `services/auth/internal/token/jwt.go`
- Create: `services/auth/internal/token/jwt_test.go`

- [ ] **Step 1: Write password hashing (argon2id)**

```go
// services/auth/internal/crypto/password.go
package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

var DefaultParams = &Argon2Params{
	Memory:      64 * 1024,
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

func HashPassword(password string) (string, error) {
	return HashPasswordWithParams(password, DefaultParams)
}

func HashPasswordWithParams(password string, p *Argon2Params) (string, error) {
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format")
	}

	var version int
	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return false, err
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expectedHash)))
	return subtle.ConstantTimeCompare(hash, expectedHash) == 1, nil
}
```

- [ ] **Step 2: Write password test**

```go
// services/auth/internal/crypto/password_test.go
package crypto_test

import (
	"testing"

	"github.com/garancehq/garance/services/auth/internal/crypto"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := crypto.HashPassword("my-secure-password")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Fatal("hash should not be empty")
	}

	ok, err := crypto.VerifyPassword("my-secure-password", hash)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("password should verify")
	}

	ok, _ = crypto.VerifyPassword("wrong-password", hash)
	if ok {
		t.Error("wrong password should not verify")
	}
}

func TestDifferentHashesForSamePassword(t *testing.T) {
	h1, _ := crypto.HashPassword("same")
	h2, _ := crypto.HashPassword("same")
	if h1 == h2 {
		t.Error("hashes should differ due to random salt")
	}
}
```

- [ ] **Step 3: Write JWT token manager**

```go
// services/auth/internal/token/jwt.go
package token

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Manager struct {
	secret        []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

type Claims struct {
	jwt.RegisteredClaims
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id,omitempty"`
	Role      string `json:"role"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func NewManager(secret string) *Manager {
	return &Manager{
		secret:        []byte(secret),
		accessExpiry:  15 * time.Minute,
		refreshExpiry: 30 * 24 * time.Hour,
	}
}

func (m *Manager) GenerateAccessToken(userID uuid.UUID, projectID, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiry)),
			Issuer:    "garance",
		},
		UserID:    userID.String(),
		ProjectID: projectID,
		Role:      role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *Manager) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

- [ ] **Step 4: Write JWT test**

```go
// services/auth/internal/token/jwt_test.go
package token_test

import (
	"testing"

	"github.com/garancehq/garance/services/auth/internal/token"
	"github.com/google/uuid"
)

func TestGenerateAndValidate(t *testing.T) {
	mgr := token.NewManager("test-secret")
	userID := uuid.New()

	accessToken, err := mgr.GenerateAccessToken(userID, "", "user")
	if err != nil {
		t.Fatal(err)
	}

	claims, err := mgr.ValidateAccessToken(accessToken)
	if err != nil {
		t.Fatal(err)
	}

	if claims.UserID != userID.String() {
		t.Errorf("expected user_id %s, got %s", userID, claims.UserID)
	}
	if claims.Role != "user" {
		t.Errorf("expected role user, got %s", claims.Role)
	}
}

func TestInvalidToken(t *testing.T) {
	mgr := token.NewManager("test-secret")
	_, err := mgr.ValidateAccessToken("invalid-token")
	if err == nil {
		t.Error("should fail for invalid token")
	}
}

func TestWrongSecret(t *testing.T) {
	mgr1 := token.NewManager("secret-1")
	mgr2 := token.NewManager("secret-2")

	userID := uuid.New()
	tok, _ := mgr1.GenerateAccessToken(userID, "user")
	_, err := mgr2.ValidateAccessToken(tok)
	if err == nil {
		t.Error("should fail with wrong secret")
	}
}

func TestRefreshTokenGeneration(t *testing.T) {
	t1, err := token.GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	t2, _ := token.GenerateRefreshToken()

	if len(t1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(t1))
	}
	if t1 == t2 {
		t.Error("refresh tokens should be unique")
	}
}
```

- [ ] **Step 5: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/auth && go test ./internal/crypto/ ./internal/token/ -v`
Expected: 6 tests pass. No Docker needed.

- [ ] **Step 6: Commit**

```bash
git add services/
git commit -m ":sparkles: feat(auth): add argon2id password hashing and JWT token management"
```

---

## Task 4: Auth Service Layer

**Files:**
- Create: `services/auth/internal/service/auth.go`
- Create: `services/auth/internal/service/auth_test.go`

- [ ] **Step 1: Write auth service**

```go
// services/auth/internal/service/auth.go
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/garancehq/garance/services/auth/internal/crypto"
	"github.com/garancehq/garance/services/auth/internal/store"
	"github.com/garancehq/garance/services/auth/internal/token"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserBanned         = errors.New("user is banned")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrPasswordRequired   = errors.New("password is required")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
)

type AuthService struct {
	db       *store.DB
	tokens   *token.Manager
}

func NewAuthService(db *store.DB, tokens *token.Manager) *AuthService {
	return &AuthService{db: db, tokens: tokens}
}

type AuthResult struct {
	User      *store.User      `json:"user"`
	TokenPair *token.TokenPair `json:"token_pair"`
}

func (s *AuthService) SignUp(ctx context.Context, email, password, userAgent, ip string) (*AuthResult, error) {
	if password == "" {
		return nil, ErrPasswordRequired
	}
	if len(password) < 8 {
		return nil, ErrWeakPassword
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := s.db.CreateUser(ctx, email, &hash)
	if err != nil {
		return nil, err
	}

	pair, err := s.createTokenPair(ctx, user, userAgent, ip)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: user, TokenPair: pair}, nil
}

func (s *AuthService) SignIn(ctx context.Context, email, password, userAgent, ip string) (*AuthResult, error) {
	user, err := s.db.GetUserByEmail(ctx, email)
	if errors.Is(err, store.ErrUserNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if user.BannedAt != nil {
		return nil, ErrUserBanned
	}

	if user.EncryptedPassword == nil {
		return nil, ErrInvalidCredentials
	}

	ok, err := crypto.VerifyPassword(password, *user.EncryptedPassword)
	if err != nil || !ok {
		return nil, ErrInvalidCredentials
	}

	pair, err := s.createTokenPair(ctx, user, userAgent, ip)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: user, TokenPair: pair}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenStr, userAgent, ip string) (*AuthResult, error) {
	session, err := s.db.GetSessionByRefreshToken(ctx, refreshTokenStr)
	if errors.Is(err, store.ErrSessionNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	// Rotate: revoke old session
	if err := s.db.RevokeSession(ctx, session.ID); err != nil {
		return nil, err
	}

	user, err := s.db.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	if user.BannedAt != nil {
		return nil, ErrUserBanned
	}

	pair, err := s.createTokenPair(ctx, user, userAgent, ip)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: user, TokenPair: pair}, nil
}

func (s *AuthService) SignOut(ctx context.Context, refreshTokenStr string) error {
	session, err := s.db.GetSessionByRefreshToken(ctx, refreshTokenStr)
	if err != nil {
		return nil // Silent — don't leak whether token was valid
	}
	return s.db.RevokeSession(ctx, session.ID)
}

func (s *AuthService) GetUser(ctx context.Context, userID uuid.UUID) (*store.User, error) {
	return s.db.GetUserByID(ctx, userID)
}

func (s *AuthService) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	return s.db.DeleteUser(ctx, userID)
}

func (s *AuthService) createTokenPair(ctx context.Context, user *store.User, userAgent, ip string) (*token.TokenPair, error) {
	accessToken, err := s.tokens.GenerateAccessToken(user.ID, "", user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := token.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	_, err = s.db.CreateSession(ctx, user.ID, refreshToken, userAgent, ip)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &token.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes in seconds
		TokenType:    "Bearer",
	}, nil
}
```

- [ ] **Step 2: Write integration test**

```go
// services/auth/internal/service/auth_test.go
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/garancehq/garance/services/auth/internal/service"
	"github.com/garancehq/garance/services/auth/internal/store"
	"github.com/garancehq/garance/services/auth/internal/token"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupAuth(t *testing.T) *service.AuthService {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}
	t.Cleanup(func() { pgContainer.Terminate(ctx) })

	connStr, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
	db, err := store.NewDB(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.RunMigrationsFromDir(ctx, "../../migrations"); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	mgr := token.NewManager("test-jwt-secret")
	return service.NewAuthService(db, mgr)
}

func TestSignUpAndSignIn(t *testing.T) {
	auth := setupAuth(t)
	ctx := context.Background()

	result, err := auth.SignUp(ctx, "test@example.fr", "password123", "TestAgent", "127.0.0.1")
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if result.User.Email != "test@example.fr" {
		t.Error("email mismatch")
	}
	if result.TokenPair.AccessToken == "" {
		t.Error("access token should not be empty")
	}
	if result.TokenPair.RefreshToken == "" {
		t.Error("refresh token should not be empty")
	}

	result2, err := auth.SignIn(ctx, "test@example.fr", "password123", "TestAgent", "127.0.0.1")
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if result2.User.ID != result.User.ID {
		t.Error("user IDs should match")
	}
}

func TestSignInWrongPassword(t *testing.T) {
	auth := setupAuth(t)
	ctx := context.Background()

	auth.SignUp(ctx, "wrong@example.fr", "password123", "", "")
	_, err := auth.SignIn(ctx, "wrong@example.fr", "badpassword", "", "")
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestSignUpWeakPassword(t *testing.T) {
	auth := setupAuth(t)
	ctx := context.Background()

	_, err := auth.SignUp(ctx, "weak@example.fr", "short", "", "")
	if err != service.ErrWeakPassword {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}

func TestRefreshTokenRotation(t *testing.T) {
	auth := setupAuth(t)
	ctx := context.Background()

	result, _ := auth.SignUp(ctx, "refresh@example.fr", "password123", "", "")
	oldRefresh := result.TokenPair.RefreshToken

	result2, err := auth.RefreshToken(ctx, oldRefresh, "", "")
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if result2.TokenPair.RefreshToken == oldRefresh {
		t.Error("refresh token should rotate")
	}

	// Old token should no longer work
	_, err = auth.RefreshToken(ctx, oldRefresh, "", "")
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials for reused token, got %v", err)
	}
}

func TestSignOut(t *testing.T) {
	auth := setupAuth(t)
	ctx := context.Background()

	result, _ := auth.SignUp(ctx, "signout@example.fr", "password123", "", "")
	err := auth.SignOut(ctx, result.TokenPair.RefreshToken)
	if err != nil {
		t.Fatalf("SignOut: %v", err)
	}

	// Token should no longer work
	_, err = auth.RefreshToken(ctx, result.TokenPair.RefreshToken, "", "")
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials after signout, got %v", err)
	}
}

func TestDuplicateSignUp(t *testing.T) {
	auth := setupAuth(t)
	ctx := context.Background()

	auth.SignUp(ctx, "dup@example.fr", "password123", "", "")
	_, err := auth.SignUp(ctx, "dup@example.fr", "password456", "", "")
	if err != store.ErrEmailAlreadyTaken {
		t.Errorf("expected ErrEmailAlreadyTaken, got %v", err)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/auth && go test ./internal/service/ -v -count=1`
Expected: 6 tests pass. Requires Docker.

- [ ] **Step 4: Commit**

```bash
git add services/
git commit -m ":sparkles: feat(auth): add auth service with signup, signin, refresh, signout"
```

---

## Task 5: HTTP Handlers

**Files:**
- Create: `services/auth/internal/handler/handler.go`
- Create: `services/auth/internal/handler/middleware.go`
- Create: `services/auth/internal/handler/response.go`
- Modify: `services/auth/main.go`

- [ ] **Step 1: Write response helpers**

```go
// services/auth/internal/handler/response.go
package handler

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Status  int         `json:"status"`
	Details interface{} `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code string, message string, status int) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorBody{Code: code, Message: message, Status: status},
	})
}
```

- [ ] **Step 2: Write auth middleware**

```go
// services/auth/internal/handler/middleware.go
package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/garancehq/garance/services/auth/internal/token"
)

type contextKey string

const UserIDKey contextKey = "user_id"
const RoleKey contextKey = "role"

func AuthMiddleware(mgr *token.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, "UNAUTHORIZED", "missing authorization header", 401)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				writeError(w, "UNAUTHORIZED", "invalid authorization format", 401)
				return
			}

			claims, err := mgr.ValidateAccessToken(parts[1])
			if err != nil {
				writeError(w, "UNAUTHORIZED", "invalid or expired token", 401)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

- [ ] **Step 3: Write auth handlers**

```go
// services/auth/internal/handler/handler.go
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/garancehq/garance/services/auth/internal/service"
	"github.com/garancehq/garance/services/auth/internal/store"
	"github.com/garancehq/garance/services/auth/internal/token"
	"github.com/google/uuid"
)

type AuthHandler struct {
	auth   *service.AuthService
	tokens *token.Manager
}

func NewAuthHandler(auth *service.AuthService, tokens *token.Manager) *AuthHandler {
	return &AuthHandler{auth: auth, tokens: tokens}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/v1/signup", h.SignUp)
	mux.HandleFunc("POST /auth/v1/signin", h.SignIn)
	mux.HandleFunc("POST /auth/v1/token/refresh", h.RefreshToken)
	mux.HandleFunc("POST /auth/v1/signout", h.SignOut)

	// Protected routes
	protected := http.NewServeMux()
	protected.HandleFunc("GET /auth/v1/user", h.GetUser)
	protected.HandleFunc("DELETE /auth/v1/user", h.DeleteUser)

	middleware := AuthMiddleware(h.tokens)
	mux.Handle("GET /auth/v1/user", middleware(protected))
	mux.Handle("DELETE /auth/v1/user", middleware(protected))
}

type signUpRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) SignUp(w http.ResponseWriter, r *http.Request) {
	var req signUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	if req.Email == "" {
		writeError(w, "VALIDATION_ERROR", "email is required", 400)
		return
	}

	result, err := h.auth.SignUp(r.Context(), req.Email, req.Password, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

type signInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) SignIn(w http.ResponseWriter, r *http.Request) {
	var req signInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	result, err := h.auth.SignIn(r.Context(), req.Email, req.Password, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	result, err := h.auth.RefreshToken(r.Context(), req.RefreshToken, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type signOutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) SignOut(w http.ResponseWriter, r *http.Request) {
	var req signOutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	h.auth.SignOut(r.Context(), req.RefreshToken)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		writeError(w, "UNAUTHORIZED", "missing user context", 401)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, "UNAUTHORIZED", "invalid user id", 401)
		return
	}

	user, err := h.auth.GetUser(r.Context(), userID)
	if err != nil {
		writeError(w, "NOT_FOUND", "user not found", 404)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		writeError(w, "UNAUTHORIZED", "missing user context", 401)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, "UNAUTHORIZED", "invalid user id", 401)
		return
	}

	if err := h.auth.DeleteUser(r.Context(), userID); err != nil {
		writeError(w, "INTERNAL_ERROR", "failed to delete user", 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		writeError(w, "UNAUTHORIZED", err.Error(), 401)
	case errors.Is(err, service.ErrUserBanned):
		writeError(w, "PERMISSION_DENIED", err.Error(), 403)
	case errors.Is(err, store.ErrEmailAlreadyTaken):
		writeError(w, "CONFLICT", err.Error(), 409)
	case errors.Is(err, service.ErrPasswordRequired), errors.Is(err, service.ErrWeakPassword):
		writeError(w, "VALIDATION_ERROR", err.Error(), 400)
	default:
		writeError(w, "INTERNAL_ERROR", "internal server error", 500)
	}
}
```

- [ ] **Step 4: Update main.go**

```go
// services/auth/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/garancehq/garance/services/auth/internal/config"
	"github.com/garancehq/garance/services/auth/internal/handler"
	"github.com/garancehq/garance/services/auth/internal/service"
	"github.com/garancehq/garance/services/auth/internal/store"
	"github.com/garancehq/garance/services/auth/internal/token"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	db, err := store.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrationsFromDir(ctx, "migrations"); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	tokenMgr := token.NewManager(cfg.JWTSecret)
	authService := service.NewAuthService(db, tokenMgr)
	authHandler := handler.NewAuthHandler(authService, tokenMgr)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	authHandler.RegisterRoutes(mux)

	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		server.Shutdown(ctx)
	}()

	log.Printf("garance auth service listening on %s", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/auth && go build ./...`
Expected: Compiles.

- [ ] **Step 6: Commit**

```bash
git add services/
git commit -m ":sparkles: feat(auth): add HTTP handlers with signup, signin, refresh, signout, get/delete user"
```

---

## Task 6: Dockerfile

**Files:**
- Create: `services/auth/Dockerfile`

- [ ] **Step 1: Write Dockerfile**

```dockerfile
# services/auth/Dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /garance-auth .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /garance-auth /usr/local/bin/garance-auth
COPY migrations/ /app/migrations/

WORKDIR /app
ENV LISTEN_ADDR=0.0.0.0:4001
EXPOSE 4001

CMD ["garance-auth"]
```

- [ ] **Step 2: Build and verify**

Run: `cd /Users/jh3ady/Development/Projects/garance/services/auth && docker build -t garance-auth:dev .`
Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add services/auth/Dockerfile
git commit -m ":whale: build(auth): add multi-stage Dockerfile"
```

---

## Summary

| Task | Description | Tests |
|---|---|---|
| 1 | Go module & project structure | 0 (build only) |
| 2 | Database schema, migrations, store layer | 6 |
| 3 | Password hashing (argon2id) & JWT tokens | 6 |
| 4 | Auth service (signup, signin, refresh, signout) | 6 |
| 5 | HTTP handlers & middleware | 0 (build + manual test) |
| 6 | Dockerfile | 0 (build only) |
| **Total** | | **18** |

### Not in this plan (deferred)

- Magic link authentication (requires email sending infrastructure — Plan 5 CLI will set up Mailhog for dev)
- OAuth2 providers (Google, GitHub, GitLab) — requires callback URLs and provider registration
- Email verification flow — requires email templates and sending
- gRPC interface (added when Gateway is built — Plan 4)
- Rate limiting on auth endpoints
