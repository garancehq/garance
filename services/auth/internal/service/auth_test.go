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
