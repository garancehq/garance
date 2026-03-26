package store_test

import (
	"context"
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

func TestProviderCRUD(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	p, err := db.CreateProvider(ctx, "google", "client-123", "encrypted-secret", "email,profile")
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	if p.Provider != "google" {
		t.Errorf("expected google, got %s", p.Provider)
	}

	found, err := db.GetProvider(ctx, "google")
	if err != nil {
		t.Fatalf("GetProvider: %v", err)
	}
	if found.ClientID != "client-123" {
		t.Error("client_id mismatch")
	}

	providers, err := db.ListProviders(ctx)
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(providers))
	}

	err = db.DeleteProvider(ctx, "google")
	if err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}
	_, err = db.GetProvider(ctx, "google")
	if err != store.ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestDuplicateProvider(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	db.CreateProvider(ctx, "github", "id1", "secret1", "user:email")
	_, err := db.CreateProvider(ctx, "github", "id2", "secret2", "user:email")
	if err != store.ErrProviderAlreadyExists {
		t.Errorf("expected ErrProviderAlreadyExists, got %v", err)
	}
}

func TestOAuthState(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	err := db.CreateOAuthState(ctx, "random-state-123", "google", "http://localhost:3000/callback")
	if err != nil {
		t.Fatalf("CreateOAuthState: %v", err)
	}

	state, err := db.GetAndConsumeOAuthState(ctx, "random-state-123")
	if err != nil {
		t.Fatalf("GetAndConsumeOAuthState: %v", err)
	}
	if state.Provider != "google" {
		t.Errorf("expected google, got %s", state.Provider)
	}

	// Consumed — second call should fail
	_, err = db.GetAndConsumeOAuthState(ctx, "random-state-123")
	if err != store.ErrOAuthStateNotFound {
		t.Errorf("expected ErrOAuthStateNotFound, got %v", err)
	}
}
