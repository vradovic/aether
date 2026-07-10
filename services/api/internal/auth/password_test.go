package auth

import (
	"errors"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	password := "correct horse battery staple"

	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}
	if hash == password {
		t.Fatal("hashPassword() returned the plaintext password")
	}

	t.Run("correct password", func(t *testing.T) {
		if err := verifyPassword(password, hash); err != nil {
			t.Fatalf("verifyPassword() error = %v", err)
		}
	})

	t.Run("incorrect password", func(t *testing.T) {
		err := verifyPassword("wrong password", hash)
		if !errors.Is(err, errInvalidCredentials) {
			t.Fatalf("verifyPassword() error = %v, want %v", err, errInvalidCredentials)
		}
	})

	t.Run("malformed hash", func(t *testing.T) {
		err := verifyPassword(password, "not-a-bcrypt-hash")
		if !errors.Is(err, errInvalidCredentials) {
			t.Fatalf("verifyPassword() error = %v, want %v", err, errInvalidCredentials)
		}
	})
}
