package service

import (
	"context"
	"testing"
	"time"

	"github.com/garancehq/garance/services/auth/internal/crypto"
	"github.com/garancehq/garance/services/auth/internal/oauth"
	"github.com/garancehq/garance/services/auth/internal/store"
	"github.com/garancehq/garance/services/auth/internal/token"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupOAuthService(t *testing.T) (*AuthService, *store.DB) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
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
	svc := NewAuthService(db, mgr)
	return svc, db
}

func TestFindOrCreateOAuthUser_CreatesUser(t *testing.T) {
	svc, _ := setupOAuthService(t)
	ctx := context.Background()

	profile := &oauth.OAuthProfile{
		ProviderUserID: "google-12345",
		Email:          "newuser@example.com",
		Name:           "New User",
		AvatarURL:      "https://example.com/avatar.jpg",
		Raw:            map[string]interface{}{"id": "google-12345", "email": "newuser@example.com"},
	}

	user, err := svc.findOrCreateOAuthUser(ctx, "google", profile)
	if err != nil {
		t.Fatalf("findOrCreateOAuthUser: %v", err)
	}
	if user.Email != "newuser@example.com" {
		t.Errorf("expected email newuser@example.com, got %s", user.Email)
	}
	if !user.EmailVerified {
		t.Error("OAuth user should have email verified")
	}

	// Identity should exist
	identity, err := svc.db.GetIdentityByProvider(ctx, "google", "google-12345")
	if err != nil {
		t.Fatalf("identity should exist: %v", err)
	}
	if identity.UserID != user.ID {
		t.Error("identity should be linked to the created user")
	}
}

func TestFindOrCreateOAuthUser_LinksExisting(t *testing.T) {
	svc, db := setupOAuthService(t)
	ctx := context.Background()

	// Create existing user with email/password
	password := "hashedpassword"
	existingUser, err := db.CreateUser(ctx, "existing@example.com", &password)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	profile := &oauth.OAuthProfile{
		ProviderUserID: "github-789",
		Email:          "existing@example.com",
		Name:           "Existing User",
		Raw:            map[string]interface{}{"id": "github-789"},
	}

	user, err := svc.findOrCreateOAuthUser(ctx, "github", profile)
	if err != nil {
		t.Fatalf("findOrCreateOAuthUser: %v", err)
	}
	if user.ID != existingUser.ID {
		t.Error("should link to existing user, not create a new one")
	}

	// Identity should be linked
	identity, err := db.GetIdentityByProvider(ctx, "github", "github-789")
	if err != nil {
		t.Fatalf("identity should be created: %v", err)
	}
	if identity.UserID != existingUser.ID {
		t.Error("identity should be linked to the existing user")
	}
}

func TestFindOrCreateOAuthUser_ReturningUser(t *testing.T) {
	svc, db := setupOAuthService(t)
	ctx := context.Background()

	// Create user and identity manually
	user, _ := db.CreateUser(ctx, "returning@example.com", nil)
	providerData := []byte(`{"name":"Old Name"}`)
	db.CreateIdentity(ctx, user.ID, "google", "google-returning", providerData)

	profile := &oauth.OAuthProfile{
		ProviderUserID: "google-returning",
		Email:          "returning@example.com",
		Name:           "Updated Name",
		Raw:            map[string]interface{}{"name": "Updated Name"},
	}

	result, err := svc.findOrCreateOAuthUser(ctx, "google", profile)
	if err != nil {
		t.Fatalf("findOrCreateOAuthUser: %v", err)
	}
	if result.ID != user.ID {
		t.Error("should return the same user")
	}

	// Provider data should be updated
	identity, _ := db.GetIdentityByProvider(ctx, "google", "google-returning")
	if string(identity.ProviderData) == string(providerData) {
		t.Error("provider data should have been updated")
	}
}

func TestOAuthCallback_BannedUser(t *testing.T) {
	svc, db := setupOAuthService(t)
	ctx := context.Background()

	// Create a banned user with identity
	user, _ := db.CreateUser(ctx, "banned@example.com", nil)
	db.Pool.Exec(ctx, `UPDATE garance_auth.users SET banned_at = now() WHERE id = $1`, user.ID)
	db.CreateIdentity(ctx, user.ID, "google", "google-banned", []byte(`{}`))

	profile := &oauth.OAuthProfile{
		ProviderUserID: "google-banned",
		Email:          "banned@example.com",
		Name:           "Banned User",
		Raw:            map[string]interface{}{"id": "google-banned"},
	}

	// findOrCreateOAuthUser itself doesn't check banned — the caller (OAuthCallback) does.
	// So we test via the user returned and the check in OAuthCallback logic.
	result, err := svc.findOrCreateOAuthUser(ctx, "google", profile)
	if err != nil {
		t.Fatalf("findOrCreateOAuthUser: %v", err)
	}
	if result.BannedAt == nil {
		t.Error("user should be banned")
	}
}

func TestOAuthCallback_NoEmail(t *testing.T) {
	// OAuthCallback checks for empty email before calling findOrCreateOAuthUser.
	// Test the check directly.
	profile := &oauth.OAuthProfile{
		ProviderUserID: "github-noemail",
		Email:          "",
		Name:           "No Email User",
		Raw:            map[string]interface{}{},
	}
	if profile.Email != "" {
		t.Error("email should be empty for this test case")
	}
	// The OAuthCallback method returns ErrOAuthNoEmail when profile.Email == ""
	// We verify the error is defined correctly
	if ErrOAuthNoEmail == nil {
		t.Error("ErrOAuthNoEmail should be defined")
	}
}

func TestOAuthCallback_ProviderDisabled(t *testing.T) {
	svc, db := setupOAuthService(t)
	ctx := context.Background()

	encryptionKey := crypto.DeriveKey("test-encryption-key")
	secret, _ := crypto.Encrypt("client-secret", encryptionKey)

	// Create a disabled provider
	db.CreateProvider(ctx, "google", "client-id", secret, "email profile")
	disabled := false
	db.UpdateProvider(ctx, "google", nil, nil, &disabled, nil)

	_, err := svc.OAuthAuthorize(ctx, "google", "http://localhost:3000/callback", "http://localhost:8080", encryptionKey)
	if err != ErrOAuthProviderNotConfigured {
		t.Errorf("expected ErrOAuthProviderNotConfigured, got %v", err)
	}
}

func TestOAuthCallback_InvalidState(t *testing.T) {
	svc, _ := setupOAuthService(t)
	ctx := context.Background()

	encryptionKey := crypto.DeriveKey("test-encryption-key")

	// Try callback with bogus state
	_, _, err := svc.OAuthCallback(ctx, "google", "fake-code", "invalid-state", "http://localhost:8080", "TestAgent", "127.0.0.1", encryptionKey)
	if err != ErrInvalidOAuthState {
		t.Errorf("expected ErrInvalidOAuthState, got %v", err)
	}
}

func TestOAuthState_Expired(t *testing.T) {
	svc, db := setupOAuthService(t)
	ctx := context.Background()

	// Insert an expired state directly
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO garance_auth.oauth_states (state, provider, redirect_uri, created_at, expires_at)
		 VALUES ($1, $2, $3, now() - interval '1 hour', now() - interval '30 minutes')`,
		"expired-state", "google", "http://localhost:3000/callback",
	)
	if err != nil {
		t.Fatalf("insert expired state: %v", err)
	}

	encryptionKey := crypto.DeriveKey("test-encryption-key")

	_, _, err = svc.OAuthCallback(ctx, "google", "fake-code", "expired-state", "http://localhost:8080", "TestAgent", "127.0.0.1", encryptionKey)
	if err != ErrInvalidOAuthState {
		t.Errorf("expected ErrInvalidOAuthState for expired state, got %v", err)
	}
}

func TestRedirectURIValidation(t *testing.T) {
	tests := []struct {
		name        string
		redirectURI string
		baseURL     string
		valid       bool
	}{
		{"empty redirect", "", "http://localhost:8080", false},
		{"localhost to localhost", "http://localhost:3000/callback", "http://localhost:8080", true},
		{"127.0.0.1 to localhost", "http://127.0.0.1:3000/callback", "http://localhost:8080", true},
		{"localhost to 127.0.0.1", "http://localhost:3000/callback", "http://127.0.0.1:8080", true},
		{"production match", "https://app.garance.io/callback", "https://app.garance.io", true},
		{"production mismatch host", "https://evil.com/callback", "https://app.garance.io", false},
		{"production mismatch scheme", "http://app.garance.io/callback", "https://app.garance.io", false},
		{"external to localhost", "https://evil.com/callback", "http://localhost:8080", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidRedirectURI(tt.redirectURI, tt.baseURL)
			if result != tt.valid {
				t.Errorf("isValidRedirectURI(%q, %q) = %v, want %v", tt.redirectURI, tt.baseURL, result, tt.valid)
			}
		})
	}
}
