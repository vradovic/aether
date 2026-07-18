package core_test

import (
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vradovic/aether/services/api/internal/core"
)

const (
	tokenUnitSigningKey = "0123456789abcdef0123456789abcdef"
	tokenUnitUserID     = "550e8400-e29b-41d4-a716-446655440000"
)

func TestIssueToken(t *testing.T) {
	token, err := core.IssueToken(tokenUnitSigningKey, tokenUnitUserID)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("IssueToken() returned an empty token")
	}

	subject, err := core.ParseTokenSubject(token, tokenUnitSigningKey)
	if err != nil {
		t.Fatalf("ParseTokenSubject() error = %v", err)
	}
	if subject != tokenUnitUserID {
		t.Fatalf("token subject = %q, want %q", subject, tokenUnitUserID)
	}
}

func TestExtractBearerToken(t *testing.T) {
	t.Run("valid headers", func(t *testing.T) {
		tests := []struct {
			name   string
			header string
			want   string
		}{
			{name: "standard", header: "Bearer abc.def.ghi", want: "abc.def.ghi"},
			{name: "case insensitive scheme", header: "bEaReR token", want: "token"},
			{name: "surrounding whitespace", header: "  Bearer token\t", want: "token"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := core.ExtractBearerToken(tt.header)
				if err != nil {
					t.Fatalf("ExtractBearerToken() error = %v", err)
				}
				if got != tt.want {
					t.Fatalf("ExtractBearerToken() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("invalid headers", func(t *testing.T) {
		tests := []struct {
			name   string
			header string
		}{
			{name: "empty", header: ""},
			{name: "scheme only", header: "Bearer"},
			{name: "empty token", header: "Bearer "},
			{name: "wrong scheme", header: "Basic token"},
			{name: "missing scheme", header: "token"},
			{name: "multiple spaces", header: "Bearer  token"},
			{name: "tab separator", header: "Bearer\ttoken"},
			{name: "space in token", header: "Bearer token extra"},
			{name: "newline in token", header: "Bearer token\nextra"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := core.ExtractBearerToken(tt.header)
				if !errors.Is(err, core.ErrInvalidAuthorization) {
					t.Fatalf("ExtractBearerToken() error = %v, want %v", err, core.ErrInvalidAuthorization)
				}
				if got != "" {
					t.Fatalf("ExtractBearerToken() = %q, want empty token", got)
				}
			})
		}
	})
}

func TestParseTokenClaims(t *testing.T) {
	token := signTokenForTest(t, jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: tokenUnitUserID})

	claims, err := core.ParseTokenClaims(token, tokenUnitSigningKey)
	if err != nil {
		t.Fatalf("ParseTokenClaims() error = %v", err)
	}
	registeredClaims, ok := claims.(*jwt.RegisteredClaims)
	if !ok {
		t.Fatalf("ParseTokenClaims() claims type = %T, want *jwt.RegisteredClaims", claims)
	}
	if registeredClaims.Subject != tokenUnitUserID {
		t.Fatalf("claims subject = %q, want %q", registeredClaims.Subject, tokenUnitUserID)
	}
}

func TestParseTokenClaimsRejectsInvalidTokens(t *testing.T) {
	tests := []struct {
		name       string
		token      func(*testing.T) string
		signingKey string
	}{
		{
			name: "wrong signing key",
			token: func(t *testing.T) string {
				return signTokenForTest(t, jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: tokenUnitUserID})
			},
			signingKey: "wrong-signing-key",
		},
		{
			name:       "malformed token",
			token:      func(*testing.T) string { return "not-a-jwt" },
			signingKey: tokenUnitSigningKey,
		},
		{
			name: "disallowed signing method",
			token: func(t *testing.T) string {
				return signTokenForTest(t, jwt.SigningMethodHS384, jwt.RegisteredClaims{Subject: tokenUnitUserID})
			},
			signingKey: tokenUnitSigningKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := core.ParseTokenClaims(tt.token(t), tt.signingKey); err == nil {
				t.Fatal("ParseTokenClaims() error = nil, want an error")
			}
		})
	}
}

func TestParseTokenSubject(t *testing.T) {
	t.Run("returns subject", func(t *testing.T) {
		token := signTokenForTest(t, jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: tokenUnitUserID})

		subject, err := core.ParseTokenSubject(token, tokenUnitSigningKey)
		if err != nil {
			t.Fatalf("ParseTokenSubject() error = %v", err)
		}
		if subject != tokenUnitUserID {
			t.Fatalf("ParseTokenSubject() = %q, want %q", subject, tokenUnitUserID)
		}
	})

	t.Run("returns empty subject when claim is absent", func(t *testing.T) {
		token := signTokenForTest(t, jwt.SigningMethodHS256, jwt.RegisteredClaims{})

		subject, err := core.ParseTokenSubject(token, tokenUnitSigningKey)
		if err != nil {
			t.Fatalf("ParseTokenSubject() error = %v", err)
		}
		if subject != "" {
			t.Fatalf("ParseTokenSubject() = %q, want empty subject", subject)
		}
	})

	t.Run("rejects invalid signature", func(t *testing.T) {
		token := signTokenForTest(t, jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: tokenUnitUserID})

		if _, err := core.ParseTokenSubject(token, "wrong-signing-key"); err == nil {
			t.Fatal("ParseTokenSubject() error = nil, want an error")
		}
	})
}

func signTokenForTest(t *testing.T, method jwt.SigningMethod, claims jwt.Claims) string {
	t.Helper()

	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(tokenUnitSigningKey))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return token
}
