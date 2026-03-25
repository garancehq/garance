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
	tok, _ := mgr1.GenerateAccessToken(userID, "", "user")
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
