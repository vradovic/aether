package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/api"
	"github.com/vradovic/aether/services/api/internal/core"
	"github.com/vradovic/aether/services/api/internal/db"
)

const (
	testSigningKey = "secret"
	testIssuer     = "aether"
	testUserID     = "550e8400-e29b-41d4-a716-446655440000"
	testRequestID  = "7b4166d7-4772-4f3a-82a3-9066a9f179b8"
)

type fakeContactsService struct {
	requestID   pgtype.UUID
	requests    []db.ContactRequest
	sendErr     error
	getErr      error
	mutationErr error
}

func (f fakeContactsService) Send(context.Context, string, string) (pgtype.UUID, error) {
	return f.requestID, f.sendErr
}

func (f fakeContactsService) GetPendingContactRequests(context.Context, string) ([]db.ContactRequest, error) {
	return f.requests, f.getErr
}

func (f fakeContactsService) Cancel(context.Context, string, pgtype.UUID) error {
	return f.mutationErr
}

func (f fakeContactsService) Accept(context.Context, string, pgtype.UUID) error {
	return f.mutationErr
}

func (f fakeContactsService) Decline(context.Context, string, pgtype.UUID) error {
	return f.mutationErr
}

func newContactsMux(service fakeContactsService) *http.ServeMux {
	logger := slog.New(slog.DiscardHandler)
	handler := api.NewContactsHandler(service, logger)
	middleware := core.NewMiddleware(testSigningKey, testIssuer)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, middleware.Authenticate)
	return mux
}

func authenticatedRequest(t *testing.T, method, target, body string) *http.Request {
	t.Helper()

	token, err := core.NewAccessTokenIssuer(testSigningKey, testIssuer, time.Hour).Issue(testUserID)
	if err != nil {
		t.Fatalf("failed to issue access token: %v", err)
	}

	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token.Value)
	return request
}

func mustUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()

	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		t.Fatalf("failed to parse test UUID: %v", err)
	}
	return id
}

func TestSendContactRequest(t *testing.T) {
	t.Run("should create contact request", func(t *testing.T) {
		requestID := mustUUID(t, testRequestID)
		mux := newContactsMux(fakeContactsService{requestID: requestID})
		request := authenticatedRequest(t, http.MethodPost, "/contact-requests", `{"username":"johndoe"}`)
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		if response.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusCreated, response.Code, response.Body.String())
		}

		var body struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("expected JSON response, got %s", response.Body.String())
		}
		if body.ID != testRequestID {
			t.Fatalf("expected request ID %q, got %q", testRequestID, body.ID)
		}
	})

	t.Run("should return bad request", func(t *testing.T) {
		mux := newContactsMux(fakeContactsService{})
		request := authenticatedRequest(t, http.MethodPost, "/contact-requests", `{"username":""}`)
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusBadRequest, response.Code, response.Body.String())
		}
	})

	t.Run("should return not found", func(t *testing.T) {
		mux := newContactsMux(fakeContactsService{sendErr: api.ErrUserNotFound})
		request := authenticatedRequest(t, http.MethodPost, "/contact-requests", `{"username":"missing"}`)
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		if response.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusNotFound, response.Code, response.Body.String())
		}
	})
}

func TestGetPendingContactRequests(t *testing.T) {
	t.Run("should return contact requests", func(t *testing.T) {
		requestID := mustUUID(t, testRequestID)
		userID := mustUUID(t, testUserID)
		mux := newContactsMux(fakeContactsService{requests: []db.ContactRequest{
			{
				ID:          requestID,
				SenderID:    requestID,
				RecipientID: userID,
				Status:      "pending",
			},
		}})
		request := authenticatedRequest(t, http.MethodGet, "/contact-requests", "")
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusOK, response.Code, response.Body.String())
		}

		var body []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("expected JSON response, got %s", response.Body.String())
		}
		if len(body) != 1 || body[0].ID != testRequestID || body[0].Status != "pending" {
			t.Fatalf("unexpected response: %s", response.Body.String())
		}
	})

	t.Run("should return internal error", func(t *testing.T) {
		mux := newContactsMux(fakeContactsService{getErr: errors.New("random error")})
		request := authenticatedRequest(t, http.MethodGet, "/contact-requests", "")
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		if response.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusInternalServerError, response.Code, response.Body.String())
		}
	})
}

func TestMutateContactRequest(t *testing.T) {
	for _, action := range []string{"cancel", "accept", "decline"} {
		t.Run(action+" should return no content", func(t *testing.T) {
			mux := newContactsMux(fakeContactsService{})
			request := authenticatedRequest(t, http.MethodPatch, "/contact-requests/"+testRequestID+"/"+action, "")
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusNoContent {
				t.Fatalf("expected status %d, got %d, body: %s", http.StatusNoContent, response.Code, response.Body.String())
			}
		})
	}

	t.Run("should return bad request for invalid ID", func(t *testing.T) {
		mux := newContactsMux(fakeContactsService{})
		request := authenticatedRequest(t, http.MethodPatch, "/contact-requests/not-a-uuid/cancel", "")
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusBadRequest, response.Code, response.Body.String())
		}
	})

	t.Run("should return not found", func(t *testing.T) {
		mux := newContactsMux(fakeContactsService{mutationErr: api.ErrRequestNotFound})
		request := authenticatedRequest(t, http.MethodPatch, "/contact-requests/"+testRequestID+"/accept", "")
		response := httptest.NewRecorder()

		mux.ServeHTTP(response, request)

		if response.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d, body: %s", http.StatusNotFound, response.Code, response.Body.String())
		}
	})
}
