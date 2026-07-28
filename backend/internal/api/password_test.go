package api_test

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/vradovic/aether/backend/internal/api"
)

func TestHashPassword(t *testing.T) {
	t.Run("hashes regular string successfully", func(t *testing.T) {
		password := "my-secure-password-123"
		hash, err := api.HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword() error = %v, want nil", err)
		}
		if hash == "" {
			t.Fatal("HashPassword() returned empty string")
		}
		if hash == password {
			t.Fatal("HashPassword() returned plaintext password")
		}

		if err := api.VerifyPassword(password, hash); err != nil {
			t.Fatalf("VerifyPassword() failed for generated hash: %v", err)
		}
	})

	t.Run("accepts 72 byte password boundary", func(t *testing.T) {
		password := strings.Repeat("a", 72)
		hash, err := api.HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword() with 72 bytes error = %v, want nil", err)
		}
		if hash == "" {
			t.Fatal("HashPassword() returned empty string for 72 byte password")
		}
	})

	t.Run("fails when password exceeds 72 bytes", func(t *testing.T) {
		password := strings.Repeat("a", 73)
		hash, err := api.HashPassword(password)
		if err == nil {
			t.Fatalf("HashPassword() expected error for >72 bytes password, got nil")
		}
		if !errors.Is(err, bcrypt.ErrPasswordTooLong) {
			t.Fatalf("HashPassword() error = %v, want %v", err, bcrypt.ErrPasswordTooLong)
		}
		if hash != "" {
			t.Fatalf("HashPassword() hash = %q, want empty string on failure", hash)
		}
	})
}

func TestVerifyPassword(t *testing.T) {
	password := "correct-horse-battery-staple"
	hash, err := api.HashPassword(password)
	if err != nil {
		t.Fatalf("setup HashPassword() error = %v", err)
	}

	t.Run("succeeds with correct password", func(t *testing.T) {
		err := api.VerifyPassword(password, hash)
		if err != nil {
			t.Fatalf("VerifyPassword() error = %v, want nil", err)
		}
	})

	t.Run("fails with wrong password and wraps ErrInvalidCredentials", func(t *testing.T) {
		wrongPassword := "wrong-password"
		err := api.VerifyPassword(wrongPassword, hash)
		if err == nil {
			t.Fatal("VerifyPassword() expected error for wrong password, got nil")
		}

		if !errors.Is(err, api.ErrInvalidCredentials) {
			t.Fatalf("VerifyPassword() error = %v, want wrapped %v", err, api.ErrInvalidCredentials)
		}
	})

	t.Run("fails with malformed hash and wraps ErrInvalidCredentials", func(t *testing.T) {
		invalidHash := "not-a-valid-bcrypt-hash"
		err := api.VerifyPassword(password, invalidHash)
		if err == nil {
			t.Fatal("VerifyPassword() expected error for invalid hash, got nil")
		}

		if !errors.Is(err, api.ErrInvalidCredentials) {
			t.Fatalf("VerifyPassword() error = %v, want wrapped %v", err, api.ErrInvalidCredentials)
		}
	})
}
