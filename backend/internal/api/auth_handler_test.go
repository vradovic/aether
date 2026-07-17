package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vradovic/aether/services/api/internal/api"
)

type fakeService struct{}

var successLoginInput = api.LoginInput{
	Email:    "john@example.com",
	Password: "thisisjohn123",
}

func (f fakeService) Login(ctx context.Context, input api.LoginInput) (api.LoginOutput, error) {
	if input == successLoginInput {
		return api.LoginOutput{
			AccessToken:      "12345",
			ExpiresInSeconds: 60,
		}, nil
	} else if input.Email == "crash@example.com" {
		return api.LoginOutput{}, errors.New("random error")
	} else {
		return api.LoginOutput{}, api.ErrInvalidCredentials
	}
}

func (f fakeService) Register(ctx context.Context, input api.RegisterInput) error {
	return nil
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			name: "valid request",
			body: `{
				"email": "johndoe@example.com",
				"password": "thisisjohn",
				"username": "johndoe",
				"firstName": "John",
				"lastName": "Doe"
			}`,
			want: http.StatusNoContent,
		},
		{
			name: "empty body",
			body: "",
			want: http.StatusBadRequest,
		},
		{
			name: "missing fields",
			body: `{
				"email": "johndoe@example.com",
				"lastName": "Doe",
			}`,
			want: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/register",
				strings.NewReader(tt.body))

			rec := httptest.NewRecorder()

			fake := fakeService{}
			logger := slog.New(slog.DiscardHandler)

			handler := api.NewAuthHandler(fake, logger)

			handler.Register(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("expected status %d, got %d, body: %s", tt.want, rec.Code, rec.Body.String())
			}
		})
	}
}

type loginResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int64  `json:"expiresIn"`
}

func TestLogin(t *testing.T) {
	t.Run("should return access token", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/login",
			strings.NewReader(fmt.Sprintf(`{"email":"%s","password":"%s"}`, successLoginInput.Email, successLoginInput.Password)),
		)

		rec := httptest.NewRecorder()

		fake := fakeService{}
		logger := slog.New(slog.DiscardHandler)

		handler := api.NewAuthHandler(fake, logger)

		handler.Login(rec, req)

		wantStatus := http.StatusOK
		gotStatus := rec.Code
		if wantStatus != gotStatus {
			t.Fatalf("expected status %d, got %d, body: %s", wantStatus, gotStatus, rec.Body.String())
		}

		var resp loginResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("expected access token but got %s", rec.Body.String())
		}
	})

	t.Run("should return unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/login",
			strings.NewReader(fmt.Sprintf(`{"email":"%s","password":"%s"}`, successLoginInput.Email, "badpass")),
		)

		rec := httptest.NewRecorder()

		fake := fakeService{}
		logger := slog.New(slog.DiscardHandler)

		handler := api.NewAuthHandler(fake, logger)

		handler.Login(rec, req)

		wantStatus := http.StatusUnauthorized
		gotStatus := rec.Code
		if wantStatus != gotStatus {
			t.Fatalf("expected status %d, got %d, body: %s", wantStatus, gotStatus, rec.Body.String())
		}
	})

	t.Run("should return bad request", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/login",
			strings.NewReader("abc"),
		)

		rec := httptest.NewRecorder()

		fake := fakeService{}
		logger := slog.New(slog.DiscardHandler)

		handler := api.NewAuthHandler(fake, logger)

		handler.Login(rec, req)

		wantStatus := http.StatusBadRequest
		gotStatus := rec.Code
		if wantStatus != gotStatus {
			t.Fatalf("expected status %d, got %d, body: %s", wantStatus, gotStatus, rec.Body.String())
		}
	})

	t.Run("should return internal error", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/login",
			strings.NewReader(fmt.Sprintf(`{"email":"%s","password":"%s"}`, "crash@example.com", successLoginInput.Password)),
		)

		rec := httptest.NewRecorder()

		fake := fakeService{}
		logger := slog.New(slog.DiscardHandler)

		handler := api.NewAuthHandler(fake, logger)

		handler.Login(rec, req)

		wantStatus := http.StatusInternalServerError
		gotStatus := rec.Code
		if wantStatus != gotStatus {
			t.Fatalf("expected status %d, got %d, body: %s", wantStatus, gotStatus, rec.Body.String())
		}
	})
}
