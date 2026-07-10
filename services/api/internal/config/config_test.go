package config

import (
	"strings"
	"testing"
	"time"
)

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("SERVER_ADDRESS", ":8080")
	t.Setenv("DB_ADDRESS", "postgres://user:password@localhost/aether")
	t.Setenv("JWT_SIGNING_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("JWT_ISSUER", "aether-test")
	t.Setenv("JWT_ACCESS_TOKEN_TTL", "30m")
}

func TestLoadJWTConfig(t *testing.T) {
	t.Run("loads required JWT settings", func(t *testing.T) {
		setRequiredEnvironment(t)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.JWTIssuer != "aether-test" {
			t.Errorf("JWTIssuer = %q, want %q", cfg.JWTIssuer, "aether-test")
		}
		if cfg.JWTAccessTokenTTL != 30*time.Minute {
			t.Errorf("JWTAccessTokenTTL = %v, want %v", cfg.JWTAccessTokenTTL, 30*time.Minute)
		}
	})

	t.Run("requires issuer", func(t *testing.T) {
		setRequiredEnvironment(t)
		t.Setenv("JWT_ISSUER", "")

		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "JWT_ISSUER is required") {
			t.Fatalf("Load() error = %v, want JWT_ISSUER required error", err)
		}
	})

	t.Run("requires access token lifetime", func(t *testing.T) {
		setRequiredEnvironment(t)
		t.Setenv("JWT_ACCESS_TOKEN_TTL", "")

		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "JWT_ACCESS_TOKEN_TTL is required") {
			t.Fatalf("Load() error = %v, want JWT_ACCESS_TOKEN_TTL required error", err)
		}
	})

	t.Run("requires signing key", func(t *testing.T) {
		setRequiredEnvironment(t)
		t.Setenv("JWT_SIGNING_KEY", "")

		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "JWT_SIGNING_KEY is required") {
			t.Fatalf("Load() error = %v, want JWT_SIGNING_KEY required error", err)
		}
	})

	t.Run("rejects a short signing key", func(t *testing.T) {
		setRequiredEnvironment(t)
		t.Setenv("JWT_SIGNING_KEY", "too-short")

		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "JWT_SIGNING_KEY") {
			t.Fatalf("Load() error = %v, want JWT_SIGNING_KEY error", err)
		}
	})

	t.Run("rejects an invalid lifetime", func(t *testing.T) {
		setRequiredEnvironment(t)
		t.Setenv("JWT_ACCESS_TOKEN_TTL", "fifteen minutes")

		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "JWT_ACCESS_TOKEN_TTL") {
			t.Fatalf("Load() error = %v, want JWT_ACCESS_TOKEN_TTL error", err)
		}
	})

	t.Run("rejects a sub-second lifetime", func(t *testing.T) {
		setRequiredEnvironment(t)
		t.Setenv("JWT_ACCESS_TOKEN_TTL", "500ms")

		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "at least one second") {
			t.Fatalf("Load() error = %v, want minimum lifetime error", err)
		}
	})
}
