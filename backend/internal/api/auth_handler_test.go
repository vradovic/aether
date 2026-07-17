package api_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vradovic/aether/services/api/internal/api"
)

type fakeService struct{}

func (f fakeService) Login(ctx context.Context, input api.LoginInput) (api.LoginOutput, error) {
	return api.LoginOutput{
		AccessToken:      "12345",
		ExpiresInSeconds: 60,
	}, nil
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
			want: http.StatusCreated,
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
				http.MethodGet,
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
