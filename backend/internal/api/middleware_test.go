package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vradovic/aether/services/api/internal/core"
)

func TestMiddlewareRequireAuth(t *testing.T) {
	const (
		signingKey = "middleware-test-signing-key"
		userID     = "550e8400-e29b-41d4-a716-446655440000"
	)

	validToken, err := core.IssueToken(signingKey, userID)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}
	tokenWithInvalidSignature, err := core.IssueToken("another-signing-key", userID)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	tests := []struct {
		name          string
		authorization string
		wantStatus    int
		wantUserID    string
	}{
		{
			name:          "passes authenticated user to next handler",
			authorization: "Bearer " + validToken,
			wantStatus:    http.StatusNoContent,
			wantUserID:    userID,
		},
		{
			name:       "rejects missing authorization header",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "rejects malformed authorization header",
			authorization: "Basic " + validToken,
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "rejects invalid token",
			authorization: "Bearer not-a-token",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "rejects token signed with another key",
			authorization: "Bearer " + tokenWithInvalidSignature,
			wantStatus:    http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			var gotUserID string
			next := func(w http.ResponseWriter, _ *http.Request, userID string) {
				called = true
				gotUserID = userID
				w.WriteHeader(http.StatusNoContent)
			}
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authorization != "" {
				request.Header.Set("Authorization", tt.authorization)
			}
			response := httptest.NewRecorder()

			Middleware{SigningKey: signingKey}.RequireAuth(next).ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusNoContent {
				if !called {
					t.Fatal("next handler was not called")
				}
				if gotUserID != tt.wantUserID {
					t.Fatalf("user ID = %q, want %q", gotUserID, tt.wantUserID)
				}
			} else if called {
				t.Fatal("next handler was called for an unauthorized request")
			}
		})
	}
}
