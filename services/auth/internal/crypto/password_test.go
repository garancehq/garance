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
