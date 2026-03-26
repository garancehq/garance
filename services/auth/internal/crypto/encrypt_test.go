package crypto_test

import (
	"testing"

	"github.com/garancehq/garance/services/auth/internal/crypto"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := crypto.DeriveKey("test-secret")
	plaintext := "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t"

	encrypted, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == plaintext {
		t.Error("encrypted should differ from plaintext")
	}

	decrypted, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != plaintext {
		t.Errorf("expected %s, got %s", plaintext, decrypted)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	key1 := crypto.DeriveKey("key-1")
	key2 := crypto.DeriveKey("key-2")

	encrypted, _ := crypto.Encrypt("secret", key1)
	_, err := crypto.Decrypt(encrypted, key2)
	if err == nil {
		t.Error("decrypt with wrong key should fail")
	}
}

func TestDifferentEncryptionsProduceDifferentCiphertexts(t *testing.T) {
	key := crypto.DeriveKey("test")
	e1, _ := crypto.Encrypt("same", key)
	e2, _ := crypto.Encrypt("same", key)
	if e1 == e2 {
		t.Error("different encryptions should produce different ciphertexts (random nonce)")
	}
}
