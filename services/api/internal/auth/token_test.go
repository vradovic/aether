package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAccessTokenIssuerIssue(t *testing.T) {
	const signingKey = "0123456789abcdef0123456789abcdef"
	const issuerName = "aether-api"
	const userID = "550e8400-e29b-41d4-a716-446655440000"

	now := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	lifetime := 15 * time.Minute
	issuer := NewAccessTokenIssuer(signingKey, issuerName, lifetime)
	issuer.now = func() time.Time { return now }

	issued, err := issuer.issue(userID)
	if err != nil {
		t.Fatalf("issue() error = %v", err)
	}
	if issued.value == "" {
		t.Fatal("issue() returned an empty token")
	}
	if issued.expiresInSeconds != 900 {
		t.Fatalf("issue() expires in = %d, want 900", issued.expiresInSeconds)
	}

	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(
		issued.value,
		claims,
		func(token *jwt.Token) (any, error) {
			return []byte(signingKey), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuerName),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("ParseWithClaims() error = %v", err)
	}
	if !parsed.Valid {
		t.Fatal("ParseWithClaims() token is invalid")
	}
	if claims.Subject != userID {
		t.Errorf("token subject = %q, want %q", claims.Subject, userID)
	}
	if claims.IssuedAt == nil || !claims.IssuedAt.Time.Equal(now) {
		t.Errorf("token issued at = %v, want %v", claims.IssuedAt, now)
	}
	wantExpiration := now.Add(lifetime)
	if claims.ExpiresAt == nil || !claims.ExpiresAt.Time.Equal(wantExpiration) {
		t.Errorf("token expiration = %v, want %v", claims.ExpiresAt, wantExpiration)
	}
}
