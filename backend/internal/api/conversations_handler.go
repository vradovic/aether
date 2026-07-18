package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/core"
)

type ConversationsService interface {
	GetConversations(ctx context.Context, userID pgtype.UUID) ([]Conversation, error)
	CreateConversation(ctx context.Context, name, userID string) (Conversation, error)
}

type conversationsHandler struct {
	svc    ConversationsService
	logger *slog.Logger
}

func NewConversationsHandler(svc ConversationsService, logger *slog.Logger) *conversationsHandler {
	return &conversationsHandler{
		svc:    svc,
		logger: logger,
	}
}

func (h *conversationsHandler) RegisterRoutes(mux *http.ServeMux, m Middleware) {
	mux.Handle("GET /conversations", m.RequireAuth(h.getConversations))
	mux.Handle("POST /conversations", m.RequireAuth(h.CreateConversation))
}

func (h *conversationsHandler) getConversations(w http.ResponseWriter, r *http.Request, userIDString string) {
	// move uuid parsing somewhere else...
	userID, err := core.ParseUUID(userIDString)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	convos, err := h.svc.GetConversations(r.Context(), userID)
	if err != nil {
		h.logger.Error("getConversations error", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	if err := json.NewEncoder(w).Encode(convos); err != nil {
		h.logger.Error("failed to encode convos", "error", err)
	}
}

type CreateConversationRequestBody struct {
	Name string `json:"name"`
}

func (h *conversationsHandler) CreateConversation(w http.ResponseWriter, r *http.Request, userID string) {
	var name string
	var body CreateConversationRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		name = ""
	} else {
		name = body.Name
	}

	convo, err := h.svc.CreateConversation(r.Context(), name, userID)
	if err != nil {
		httpInternalServerError(w)
		return
	}

	writeJSONResponse(w, http.StatusCreated, convo, h.logger)
}
