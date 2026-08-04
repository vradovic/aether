package conversations

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/vradovic/aether/backend/internal/api"
	"github.com/vradovic/aether/backend/internal/api/httputil"
	"github.com/vradovic/aether/backend/internal/core"
)

type ServiceInterface interface {
	GetConversations(context.Context, string) ([]Conversation, error)
	CreateConversation(context.Context, string, string) (Conversation, error)
	UpdateConversation(context.Context, string, string, string) (Conversation, error)
	GetMessages(context.Context, string, string, string) ([]Message, error)
	AddParticipant(context.Context, string, string, string) (ConversationParticipant, error)
	DeleteConversation(context.Context, string, string) error
	RemoveParticipant(context.Context, string, string, string) error
}

type Handler struct {
	svc    ServiceInterface
	logger *slog.Logger
}

func NewHandler(svc ServiceInterface, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, m api.Middleware) {
	mux.Handle("GET /conversations", m.RequireAuth(h.getConversations))
	mux.Handle("POST /conversations", m.RequireAuth(h.CreateConversation))
	mux.Handle("PATCH /conversations/{conversationID}", m.RequireAuth(h.updateConversation))
	mux.Handle("DELETE /conversations/{conversationID}", m.RequireAuth(h.deleteConversation))
	mux.Handle("GET /conversations/{conversationID}/messages", m.RequireAuth(h.getMessages))
	mux.Handle("POST /conversations/{conversationID}/participants", m.RequireAuth(h.addParticipant))
	mux.Handle("DELETE /conversations/{conversationID}/participants/{participantID}", m.RequireAuth(h.removeParticipant))
}

func (h *Handler) getConversations(w http.ResponseWriter, r *http.Request, userID string) {
	conversations, err := h.svc.GetConversations(r.Context(), userID)
	if err != nil {
		if errors.Is(err, core.ErrInvalidID) {
			httputil.Unauthorized(w)
			return
		}
		h.logger.Error("failed to get conversations", "error", err)
		httputil.InternalServerError(w)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, conversations, h.logger)
}

type CreateConversationRequestBody struct {
	Name string `json:"name"`
}

func (h *Handler) CreateConversation(w http.ResponseWriter, r *http.Request, userID string) {
	var body CreateConversationRequestBody
	err := httputil.DecodeJSON(w, r, &body)
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
			httputil.Unauthorized(w)
		default:
			h.logger.Error("failed to create conversation", "error", err)
			httputil.InternalServerError(w)
		}
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, conversation, h.logger)
}

type updateConversationRequest struct {
	Name string `json:"name"`
}

func (h *Handler) updateConversation(w http.ResponseWriter, r *http.Request, userID string) {
	var body updateConversationRequest
	if err := httputil.DecodeJSON(w, r, &body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	conversation, err := h.svc.UpdateConversation(r.Context(), body.Name, userID, r.PathValue("conversationID"))
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidConversationName), errors.Is(err, ErrInvalidConversationID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidID):
			httputil.Unauthorized(w)
		case errors.Is(err, ErrConversationNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			h.logger.Error("failed to update conversation", "error", err)
			httputil.InternalServerError(w)
		}
		return
	}
	httputil.WriteJSON(w, http.StatusOK, conversation, h.logger)
}

func (h *Handler) getMessages(w http.ResponseWriter, r *http.Request, userID string) {
	afterID := ""
	values, present := r.URL.Query()["after_id"]
	if present {
		if len(values) != 1 || values[0] == "" {
			http.Error(w, ErrInvalidAfterID.Error(), http.StatusBadRequest)
			return
		}
		afterID = values[0]
	}

	messages, err := h.svc.GetMessages(r.Context(), userID, r.PathValue("conversationID"), afterID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidAfterID), errors.Is(err, ErrInvalidConversationID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidID):
			httputil.Unauthorized(w)
		case errors.Is(err, ErrConversationNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			h.logger.Error("failed to get conversation messages", "error", err)
			httputil.InternalServerError(w)
		}
		return
	}
	httputil.WriteJSON(w, http.StatusOK, messages, h.logger)
}

type addParticipantRequest struct {
	UserID string `json:"userId"`
}

func (h *Handler) addParticipant(w http.ResponseWriter, r *http.Request, userID string) {
	var body addParticipantRequest
	if err := httputil.DecodeJSON(w, r, &body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	participant, err := h.svc.AddParticipant(r.Context(), userID, r.PathValue("conversationID"), body.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidConversationID), errors.Is(err, ErrInvalidParticipantID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidID):
			httputil.Unauthorized(w)
		case errors.Is(err, ErrConversationNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, ErrParticipantNotContact):
			http.Error(w, err.Error(), http.StatusForbidden)
		case errors.Is(err, ErrParticipantExists):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			h.logger.Error("failed to add conversation participant", "error", err)
			httputil.InternalServerError(w)
		}
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, participant, h.logger)
}

func (h *Handler) deleteConversation(w http.ResponseWriter, r *http.Request, userID string) {
	if err := h.svc.DeleteConversation(r.Context(), userID, r.PathValue("conversationID")); err != nil {
		switch {
		case errors.Is(err, ErrInvalidConversationID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidID):
			httputil.Unauthorized(w)
		case errors.Is(err, ErrConversationNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			h.logger.Error("failed to delete conversation", "error", err)
			httputil.InternalServerError(w)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) removeParticipant(w http.ResponseWriter, r *http.Request, userID string) {
	if err := h.svc.RemoveParticipant(r.Context(), userID, r.PathValue("conversationID"), r.PathValue("participantID")); err != nil {
		switch {
		case errors.Is(err, ErrInvalidConversationID), errors.Is(err, ErrInvalidParticipantID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidID):
			httputil.Unauthorized(w)
		case errors.Is(err, ErrParticipantNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			h.logger.Error("failed to remove conversation participant", "error", err)
			httputil.InternalServerError(w)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
