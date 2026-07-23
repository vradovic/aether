package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/vradovic/aether/services/api/internal/core"
)

type ConversationsService interface {
	GetConversations(context.Context, string) ([]Conversation, error)
	CreateConversation(context.Context, string, string) (Conversation, error)
	UpdateConversation(context.Context, string, string, string) (Conversation, error)
	GetMessages(context.Context, string, string, int64) ([]Message, error)
	AddParticipant(context.Context, string, string, string) (ConversationParticipant, error)
	DeleteConversation(context.Context, string, string) error
	RemoveParticipant(context.Context, string, string, string) error
}

type conversationsHandler struct {
	svc    ConversationsService
	logger *slog.Logger
}

func NewConversationsHandler(svc ConversationsService, logger *slog.Logger) *conversationsHandler {
	return &conversationsHandler{svc: svc, logger: logger}
}

func (h *conversationsHandler) RegisterRoutes(mux *http.ServeMux, m Middleware) {
	mux.Handle("GET /conversations", m.RequireAuth(h.getConversations))
	mux.Handle("POST /conversations", m.RequireAuth(h.CreateConversation))
	mux.Handle("PATCH /conversations/{conversationID}", m.RequireAuth(h.updateConversation))
	mux.Handle("DELETE /conversations/{conversationID}", m.RequireAuth(h.deleteConversation))
	mux.Handle("GET /conversations/{conversationID}/messages", m.RequireAuth(h.getMessages))
	mux.Handle("POST /conversations/{conversationID}/participants", m.RequireAuth(h.addParticipant))
	mux.Handle("DELETE /conversations/{conversationID}/participants/{participantID}", m.RequireAuth(h.removeParticipant))
}

func (h *conversationsHandler) getConversations(w http.ResponseWriter, r *http.Request, userID string) {
	conversations, err := h.svc.GetConversations(r.Context(), userID)
	if err != nil {
		if errors.Is(err, core.ErrInvalidID) {
			httpUnauthorized(w)
			return
		}
		h.logger.Error("failed to get conversations", "error", err)
		httpInternalServerError(w)
		return
	}
	writeJSONResponse(w, http.StatusOK, conversations, h.logger)
}

type CreateConversationRequestBody struct {
	Name string `json:"name"`
}

func (h *conversationsHandler) CreateConversation(w http.ResponseWriter, r *http.Request, userID string) {
	var body CreateConversationRequestBody
	err := decodeJSONBody(w, r, &body)
	if err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	conversation, err := h.svc.CreateConversation(r.Context(), body.Name, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidConversationName):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidID):
			httpUnauthorized(w)
		default:
			h.logger.Error("failed to create conversation", "error", err)
			httpInternalServerError(w)
		}
		return
	}
	writeJSONResponse(w, http.StatusCreated, conversation, h.logger)
}

type updateConversationRequest struct {
	Name string `json:"name"`
}

func (h *conversationsHandler) updateConversation(w http.ResponseWriter, r *http.Request, userID string) {
	var body updateConversationRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	conversation, err := h.svc.UpdateConversation(r.Context(), body.Name, userID, r.PathValue("conversationID"))
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidConversationName), errors.Is(err, ErrInvalidConversationID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidID):
			httpUnauthorized(w)
		case errors.Is(err, ErrConversationNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			h.logger.Error("failed to update conversation", "error", err)
			httpInternalServerError(w)
		}
		return
	}
	writeJSONResponse(w, http.StatusOK, conversation, h.logger)
}

func (h *conversationsHandler) getMessages(w http.ResponseWriter, r *http.Request, userID string) {
	afterSequence := int64(0)
	values, present := r.URL.Query()["after_sequence"]
	if present {
		if len(values) != 1 || values[0] == "" {
			http.Error(w, ErrInvalidAfterSequence.Error(), http.StatusBadRequest)
			return
		}
		sequence, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil || sequence < 0 {
			http.Error(w, ErrInvalidAfterSequence.Error(), http.StatusBadRequest)
			return
		}
		afterSequence = sequence
	}

	messages, err := h.svc.GetMessages(r.Context(), userID, r.PathValue("conversationID"), afterSequence)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidAfterSequence), errors.Is(err, ErrInvalidConversationID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidID):
			httpUnauthorized(w)
		case errors.Is(err, ErrConversationNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			h.logger.Error("failed to get conversation messages", "error", err)
			httpInternalServerError(w)
		}
		return
	}
	writeJSONResponse(w, http.StatusOK, messages, h.logger)
}

type addParticipantRequest struct {
	UserID string `json:"userId"`
}

func (h *conversationsHandler) addParticipant(w http.ResponseWriter, r *http.Request, userID string) {
	var body addParticipantRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	participant, err := h.svc.AddParticipant(r.Context(), userID, r.PathValue("conversationID"), body.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidConversationID), errors.Is(err, ErrInvalidParticipantID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidID):
			httpUnauthorized(w)
		case errors.Is(err, ErrConversationNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, ErrParticipantNotContact):
			http.Error(w, err.Error(), http.StatusForbidden)
		case errors.Is(err, ErrParticipantExists):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			h.logger.Error("failed to add conversation participant", "error", err)
			httpInternalServerError(w)
		}
		return
	}
	writeJSONResponse(w, http.StatusCreated, participant, h.logger)
}

func (h *conversationsHandler) deleteConversation(w http.ResponseWriter, r *http.Request, userID string) {
	if err := h.svc.DeleteConversation(r.Context(), userID, r.PathValue("conversationID")); err != nil {
		switch {
		case errors.Is(err, ErrInvalidConversationID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidID):
			httpUnauthorized(w)
		case errors.Is(err, ErrConversationNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			h.logger.Error("failed to delete conversation", "error", err)
			httpInternalServerError(w)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *conversationsHandler) removeParticipant(w http.ResponseWriter, r *http.Request, userID string) {
	if err := h.svc.RemoveParticipant(r.Context(), userID, r.PathValue("conversationID"), r.PathValue("participantID")); err != nil {
		switch {
		case errors.Is(err, ErrInvalidConversationID), errors.Is(err, ErrInvalidParticipantID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidID):
			httpUnauthorized(w)
		case errors.Is(err, ErrParticipantNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			h.logger.Error("failed to remove conversation participant", "error", err)
			httpInternalServerError(w)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
