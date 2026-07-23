package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vradovic/aether/services/api/internal/api"
	"github.com/vradovic/aether/services/api/internal/core"
)

const testConversationID = "d56ba3a9-d89e-4474-aefa-6247d34b01f9"
const testParticipantID = "b0a782cf-b3d4-4ed1-ab01-b36022c1480a"

type fakeConversationsService struct {
	conversations []api.Conversation
	messages      []api.Message
	conversation  api.Conversation
	participant   api.ConversationParticipant
	err           error
	getFn         func(context.Context, string) ([]api.Conversation, error)
	updateFn      func(context.Context, string, string, string) (api.Conversation, error)
	messagesFn    func(context.Context, string, string, int64) ([]api.Message, error)
	addFn         func(context.Context, string, string, string) (api.ConversationParticipant, error)
	deleteFn      func(context.Context, string, string) error
	removeFn      func(context.Context, string, string, string) error
}

func (f fakeConversationsService) GetConversations(ctx context.Context, userID string) ([]api.Conversation, error) {
	if f.getFn != nil {
		return f.getFn(ctx, userID)
	}
	return f.conversations, f.err
}

func (f fakeConversationsService) CreateConversation(context.Context, string, string) (api.Conversation, error) {
	return f.conversation, f.err
}

func (f fakeConversationsService) UpdateConversation(ctx context.Context, name, userID, conversationID string) (api.Conversation, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, name, userID, conversationID)
	}
	return f.conversation, f.err
}

func (f fakeConversationsService) GetMessages(ctx context.Context, userID, conversationID string, afterSequence int64) ([]api.Message, error) {
	if f.messagesFn != nil {
		return f.messagesFn(ctx, userID, conversationID, afterSequence)
	}
	return f.messages, f.err
}

func (f fakeConversationsService) AddParticipant(ctx context.Context, userID, conversationID, participantID string) (api.ConversationParticipant, error) {
	if f.addFn != nil {
		return f.addFn(ctx, userID, conversationID, participantID)
	}
	return f.participant, f.err
}

func (f fakeConversationsService) DeleteConversation(ctx context.Context, userID, conversationID string) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, userID, conversationID)
	}
	return f.err
}

func (f fakeConversationsService) RemoveParticipant(ctx context.Context, userID, conversationID, participantID string) error {
	if f.removeFn != nil {
		return f.removeFn(ctx, userID, conversationID, participantID)
	}
	return f.err
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
		mux := newConversationsMux(fakeConversationsService{getFn: func(_ context.Context, userID string) ([]api.Conversation, error) {
			if userID != testUserID {
				t.Fatalf("user ID = %q, want %q", userID, testUserID)
			}
			return []api.Conversation{{
				ID:                  testConversationID,
				Name:                "General",
				CreatedBy:           testUserID,
				LastMessageSequence: 3,
			}}, nil
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
		mux := newConversationsMux(fakeConversationsService{err: core.ErrInvalidID})
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

func TestUpdateConversation(t *testing.T) {
	t.Run("updates the name", func(t *testing.T) {
		service := fakeConversationsService{
			updateFn: func(_ context.Context, name, userID, conversationID string) (api.Conversation, error) {
				if name != "Project chat" || userID != testUserID || conversationID != testConversationID {
					t.Fatalf("unexpected update arguments: name=%q userID=%q conversationID=%q", name, userID, conversationID)
				}
				return api.Conversation{ID: conversationID, Name: name, CreatedBy: userID}, nil
			},
		}
		response := serveConversationRequest(t, service, http.MethodPatch, "/conversations/"+testConversationID, `{"name":"Project chat"}`)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var conversation api.Conversation
		if err := json.Unmarshal(response.Body.Bytes(), &conversation); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if conversation.Name != "Project chat" {
			t.Fatalf("name = %q, want Project chat", conversation.Name)
		}
	})

	for _, tt := range []struct {
		name       string
		path       string
		body       string
		err        error
		wantStatus int
	}{
		{name: "invalid conversation ID", path: "/conversations/not-a-uuid", body: `{"name":"Name"}`, err: api.ErrInvalidConversationID, wantStatus: http.StatusBadRequest},
		{name: "invalid body", path: "/conversations/" + testConversationID, body: `{"unknown":true}`, wantStatus: http.StatusBadRequest},
		{name: "invalid name", path: "/conversations/" + testConversationID, body: `{"name":""}`, err: api.ErrInvalidConversationName, wantStatus: http.StatusBadRequest},
		{name: "not found", path: "/conversations/" + testConversationID, body: `{"name":"Name"}`, err: api.ErrConversationNotFound, wantStatus: http.StatusNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := serveConversationRequest(t, fakeConversationsService{err: tt.err}, http.MethodPatch, tt.path, tt.body)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tt.wantStatus, response.Body.String())
			}
		})
	}
}

func TestGetConversationMessages(t *testing.T) {
	t.Run("passes after_sequence and returns messages", func(t *testing.T) {
		service := fakeConversationsService{
			messagesFn: func(_ context.Context, userID, conversationID string, afterSequence int64) ([]api.Message, error) {
				if userID != testUserID || conversationID != testConversationID || afterSequence != 7 {
					t.Fatalf("unexpected message arguments: userID=%q conversationID=%q after=%d", userID, conversationID, afterSequence)
				}
				return []api.Message{{ID: testParticipantID, ConversationID: conversationID, MessageSequence: 8, Body: "hello"}}, nil
			},
		}
		response := serveConversationRequest(t, service, http.MethodGet, "/conversations/"+testConversationID+"/messages?after_sequence=7", "")

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}
		var messages []api.Message
		if err := json.Unmarshal(response.Body.Bytes(), &messages); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(messages) != 1 || messages[0].MessageSequence != 8 {
			t.Fatalf("unexpected messages: %+v", messages)
		}
	})

	for _, query := range []string{"?after_sequence=-1", "?after_sequence=nope", "?after_sequence=", "?after_sequence=1&after_sequence=2"} {
		t.Run(query, func(t *testing.T) {
			response := serveConversationRequest(t, fakeConversationsService{}, http.MethodGet, "/conversations/"+testConversationID+"/messages"+query, "")
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
			}
		})
	}
}

func TestAddConversationParticipant(t *testing.T) {
	t.Run("adds a participant", func(t *testing.T) {
		service := fakeConversationsService{
			addFn: func(_ context.Context, userID, conversationID, participantID string) (api.ConversationParticipant, error) {
				if userID != testUserID || conversationID != testConversationID || participantID != testParticipantID {
					t.Fatalf("unexpected add arguments: userID=%q conversationID=%q participantID=%q", userID, conversationID, participantID)
				}
				return api.ConversationParticipant{ConversationID: conversationID, UserID: participantID}, nil
			},
		}
		response := serveConversationRequest(t, service, http.MethodPost, "/conversations/"+testConversationID+"/participants", `{"userId":"`+testParticipantID+`"}`)

		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body.String())
		}
		var participant api.ConversationParticipant
		if err := json.Unmarshal(response.Body.Bytes(), &participant); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if participant.UserID != testParticipantID {
			t.Fatalf("participant ID = %q, want %q", participant.UserID, testParticipantID)
		}
	})

	for _, tt := range []struct {
		name       string
		body       string
		err        error
		wantStatus int
	}{
		{name: "invalid participant ID", body: `{"userId":"invalid"}`, err: api.ErrInvalidParticipantID, wantStatus: http.StatusBadRequest},
		{name: "not a contact", body: `{"userId":"` + testParticipantID + `"}`, err: api.ErrParticipantNotContact, wantStatus: http.StatusForbidden},
		{name: "already present", body: `{"userId":"` + testParticipantID + `"}`, err: api.ErrParticipantExists, wantStatus: http.StatusConflict},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := serveConversationRequest(t, fakeConversationsService{err: tt.err}, http.MethodPost, "/conversations/"+testConversationID+"/participants", tt.body)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tt.wantStatus, response.Body.String())
			}
		})
	}
}

func TestDeleteConversationAndParticipant(t *testing.T) {
	for _, tt := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "conversation", method: http.MethodDelete, path: "/conversations/" + testConversationID},
		{name: "participant", method: http.MethodDelete, path: "/conversations/" + testConversationID + "/participants/" + testParticipantID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := serveConversationRequest(t, fakeConversationsService{}, tt.method, tt.path, "")
			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
			}
		})
	}

	response := serveConversationRequest(t, fakeConversationsService{err: api.ErrParticipantNotFound}, http.MethodDelete, "/conversations/"+testConversationID+"/participants/"+testParticipantID, "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}
}

func serveConversationRequest(t *testing.T, service fakeConversationsService, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := newConversationsMux(service)
	request := authenticatedRequest(t, method, path, body)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}
