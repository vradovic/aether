package auth

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/vradovic/aether/services/api/internal/db"
	"github.com/vradovic/aether/services/api/internal/shared"
)

func TestHandlerLogin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	passwordHash, err := hashPassword("password123")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	t.Run("returns access token", func(t *testing.T) {
		querier := &fakeQuerier{
			credentials: db.GetUserCredentialsByEmailRow{
				UserID:       testUserID,
				PasswordHash: passwordHash,
			},
		}
		tokens := &fakeTokenIssuer{
			token: shared.IssuedToken{Value: "signed-token", ExpiresInSeconds: 900},
		}
		h := NewHandler(&service{
			querier:     querier,
			tokenIssuer: tokens,
		}, logger)
		mux := http.NewServeMux()
		h.RegisterRoutes(mux)

		request := httptest.NewRequest(
			http.MethodPost,
			"/login",
			strings.NewReader(`{"email":"user@example.com","password":"password123"}`),
		)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
		}
		if got := response.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}
		var body loginResponse
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		want := loginResponse{AccessToken: "signed-token", TokenType: "Bearer", ExpiresIn: 900}
		if body != want {
			t.Errorf("response = %#v, want %#v", body, want)
		}
	})

	t.Run("rejects malformed JSON", func(t *testing.T) {
		h := NewHandler(&service{}, logger)
		request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":`))
		response := httptest.NewRecorder()

		h.login(response, request)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
		}
		if response.Body.String() != "invalid request body\n" {
			t.Errorf("body = %q, want %q", response.Body.String(), "invalid request body\n")
		}
	})

	t.Run("unknown email returns invalid credentials", func(t *testing.T) {
		h := NewHandler(&service{
			querier:     &fakeQuerier{credentialErr: pgx.ErrNoRows},
			tokenIssuer: &fakeTokenIssuer{},
		}, logger)
		request := httptest.NewRequest(
			http.MethodPost,
			"/login",
			strings.NewReader(`{"email":"missing@example.com","password":"password123"}`),
		)
		response := httptest.NewRecorder()

		h.login(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
		}
		if response.Body.String() != "invalid credentials\n" {
			t.Errorf("body = %q, want %q", response.Body.String(), "invalid credentials\n")
		}
	})
}

func TestHandlerRegister(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("passes username to user creation", func(t *testing.T) {
		querier := &fakeQuerier{}
		h := NewHandler(&service{querier: querier}, logger)
		request := httptest.NewRequest(
			http.MethodPost,
			"/register",
			strings.NewReader(`{"email":"user@example.com","username":"petar","password":"password123","firstName":"Petar","lastName":"Petrović"}`),
		)
		response := httptest.NewRecorder()

		h.register(response, request)

		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusCreated, response.Body.String())
		}
		if querier.params.Username != "petar" {
			t.Errorf("CreateUser() username = %q, want %q", querier.params.Username, "petar")
		}
	})

	t.Run("rejects invalid username", func(t *testing.T) {
		querier := &fakeQuerier{}
		h := NewHandler(&service{querier: querier}, logger)
		request := httptest.NewRequest(
			http.MethodPost,
			"/register",
			strings.NewReader(`{"email":"user@example.com","username":"ab","password":"password123","firstName":"Petar","lastName":"Petrović"}`),
		)
		response := httptest.NewRecorder()

		h.register(response, request)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusBadRequest, response.Body.String())
		}
		if response.Body.String() != errUsernameLength.Error()+"\n" {
			t.Errorf("body = %q, want %q", response.Body.String(), errUsernameLength.Error()+"\n")
		}
		if querier.calls != 0 {
			t.Fatalf("CreateUser() calls = %d, want 0", querier.calls)
		}
	})
}
