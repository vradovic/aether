package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/api"
	"github.com/vradovic/aether/services/api/internal/core"
)

const testConversationID = "d56ba3a9-d89e-4474-aefa-6247d34b01f9"

type fakeConversationsService struct {
	conversations []api.Conversation
	err           error
}

func (f fakeConversationsService) GetConversations(context.Context, pgtype.UUID) ([]api.Conversation, error) {
	return f.conversations, f.err
}

func (f fakeConversationsService) CreateConversation(context.Context, string, string) (api.Conversation, error) {
	return api.Conversation{}, f.err
}

func newConversationsMux(service fakeConversationsService) *http.ServeMux {
	logger := slog.New(slog.DiscardHandler)
	handler := api.NewConversationsHandler(service, logger)
	middleware := api.Middleware{SigningKey: testSigningKey}
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, middleware)
	return mux
}

func TestGetConversations(t *testing.T) {
	t.Run("should return conversations", func(t *testing.T) {
		mux := newConversationsMux(fakeConversationsService{conversations: []api.Conversation{
			{
				ID:                  testConversationID,
				Name:                "General",
				CreatedBy:           testUserID,
				LastMessageSequence: 3,
			},
		}})
		request := authenticatedRequest(t, http.MethodGet, "/conversations", "")
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusOK, response.Code, response.Body.String())
		}

		var body []api.Conversation
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("expected JSON response, got %s", response.Body.String())
		}
		if len(body) != 1 || body[0].ID != testConversationID || body[0].LastMessageSequence != 3 {
			t.Fatalf("unexpected response: %s", response.Body.String())
		}
	})

	t.Run("should return unauthorized", func(t *testing.T) {
		mux := newConversationsMux(fakeConversationsService{})
		request := httptest.NewRequest(http.MethodGet, "/conversations", nil)
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusUnauthorized, response.Code, response.Body.String())
		}
	})

	t.Run("should reject invalid user ID", func(t *testing.T) {
		token, err := core.IssueToken(testSigningKey, "not-a-uuid")
		if err != nil {
			t.Fatalf("failed to issue access token: %v", err)
		}
		mux := newConversationsMux(fakeConversationsService{})
		request := httptest.NewRequest(http.MethodGet, "/conversations", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusUnauthorized, response.Code, response.Body.String())
		}
	})

	t.Run("should return internal error", func(t *testing.T) {
		mux := newConversationsMux(fakeConversationsService{err: errors.New("random error")})
		request := authenticatedRequest(t, http.MethodGet, "/conversations", "")
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		if response.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusInternalServerError, response.Code, response.Body.String())
		}
	})
}

func TestCreateConversation(t *testing.T) {
	tests := []struct {
		name       string
		wantStatus int
		err        error
	}{
		{
			name:       "should succeed",
			wantStatus: http.StatusCreated,
			err:        nil,
		},
		{
			name:       "should unexpected error",
			wantStatus: http.StatusInternalServerError,
			err:        errors.New("whoops"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := newConversationsMux(fakeConversationsService{err: tt.err})
			req := authenticatedRequest(t, http.MethodPost, "/conversations", "")
			resp := httptest.NewRecorder()

			mux.ServeHTTP(resp, req)
			gotStatus := resp.Result().StatusCode

			if tt.wantStatus != gotStatus {
				t.Fatalf("got %d, want %d", gotStatus, tt.wantStatus)
			}
		})
	}
}
