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

func (h *conversationsHandler) RegisterRoutes(mux *http.ServeMux, authenticate func(http.Handler) http.Handler) {
	// get convos
	// create convo
	// delete convo
	// add participant
	// remove participant
	// update convo name
	// sync messages
	mux.Handle("GET /conversations", authenticate(http.HandlerFunc(h.getConversations)))
}

func (h *conversationsHandler) getConversations(w http.ResponseWriter, r *http.Request) {
	userIDString, ok := core.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

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
