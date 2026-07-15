package contacts

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/core"
)

type sendRequest struct {
	Username string `json:"username"`
}

type sendResponse struct {
	ID string `json:"id"`
}

type handler struct {
	service *service
	logger  *slog.Logger
}

func NewHandler(service *service, logger *slog.Logger) *handler {
	return &handler{service: service, logger: logger}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, authenticate func(http.Handler) http.Handler) {
	mux.Handle("POST /contact-requests", authenticate(http.HandlerFunc(h.send)))
	mux.Handle("DELETE /contact-requests/{requestID}", authenticate(http.HandlerFunc(h.cancel)))
	mux.Handle("POST /contact-requests/{requestID}/accept", authenticate(http.HandlerFunc(h.accept)))
	mux.Handle("POST /contact-requests/{requestID}/decline", authenticate(http.HandlerFunc(h.decline)))
}

func (h *handler) send(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var request sendRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Username == "" {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	userID, _ := core.UserIDFromContext(r.Context())
	requestID, err := h.service.send(r.Context(), userID, request.Username)
	if err != nil {
		h.writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(sendResponse{ID: requestID.String()}); err != nil {
		h.logger.Error("failed to encode contact request response", "err", err)
	}
}

func (h *handler) cancel(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.service.cancel)
}

func (h *handler) accept(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.service.accept)
}

func (h *handler) decline(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.service.decline)
}

func (h *handler) mutate(w http.ResponseWriter, r *http.Request, action func(context.Context, string, pgtype.UUID) error) {
	var requestID pgtype.UUID
	if err := requestID.Scan(r.PathValue("requestID")); err != nil || !requestID.Valid {
		http.Error(w, "invalid contact request ID", http.StatusBadRequest)
		return
	}
	userID, _ := core.UserIDFromContext(r.Context())
	if err := action(r.Context(), userID, requestID); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUserNotFound), errors.Is(err, ErrRequestNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrSelfRequest):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrPendingRequestExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, core.ErrInvalidID):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	default:
		h.logger.Error("contact request operation failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
