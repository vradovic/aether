package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiddlewareAuthenticate(t *testing.T) {
	const userID = "550e8400-e29b-41d4-a716-446655440000"
	issuer := NewAccessTokenIssuer("secret", "aether", time.Hour)
	issued, err := issuer.Issue(userID)
	if err != nil {
		t.Fatalf("issue() error = %v", err)
	}

	t.Run("stores authenticated user ID in context", func(t *testing.T) {
		var gotUserID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUserID, _ = UserIDFromContext(r.Context())
			w.WriteHeader(http.StatusNoContent)
		})
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set("Authorization", "Bearer "+issued.Value)
		response := httptest.NewRecorder()

		NewMiddleware("secret", "aether").Authenticate(next).ServeHTTP(response, request)

		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
		}
		if gotUserID != userID {
			t.Errorf("user ID = %q, want %q", gotUserID, userID)
		}
	})

	t.Run("rejects missing bearer token", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()
		NewMiddleware("secret", "aether").Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("next handler was called")
		})).ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
		}
	})
}
