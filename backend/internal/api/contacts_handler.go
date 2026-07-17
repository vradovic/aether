package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/core"
)

type sendRequest struct {
	Username string `json:"username"`
}

type sendResponse struct {
	ID string `json:"id"`
}

type contactRequestResponse struct {
	ID          string    `json:"id"`
	SenderID    string    `json:"senderId"`
	RecipientID string    `json:"recipientId"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type contactsHandler struct {
	service *contactsService
	logger  *slog.Logger
}

func NewContactsHandler(service *contactsService, logger *slog.Logger) *contactsHandler {
	return &contactsHandler{service: service, logger: logger}
}

func (h *contactsHandler) RegisterRoutes(mux *http.ServeMux, authenticate func(http.Handler) http.Handler) {
	mux.Handle("POST /contact-requests", authenticate(http.HandlerFunc(h.send)))
	mux.Handle("GET /contact-requests", authenticate(http.HandlerFunc(h.getPendingContactRequests)))
	mux.Handle("POST /contact-requests/{requestID}", authenticate(http.HandlerFunc(h.cancel)))
	mux.Handle("POST /contact-requests/{requestID}/accept", authenticate(http.HandlerFunc(h.accept)))
	mux.Handle("POST /contact-requests/{requestID}/decline", authenticate(http.HandlerFunc(h.decline)))
}

func (h *contactsHandler) getPendingContactRequests(w http.ResponseWriter, r *http.Request) {
	userID, _ := core.UserIDFromContext(r.Context())
	requests, err := h.service.getPendingContactRequests(r.Context(), userID)
	if err != nil {
		h.writeError(w, err)
		return
	}

	response := make([]contactRequestResponse, len(requests))
	for i, request := range requests {
		response[i] = contactRequestResponse{
			ID:          request.ID.String(),
			SenderID:    request.SenderID.String(),
			RecipientID: request.RecipientID.String(),
			Status:      request.Status,
			CreatedAt:   request.CreatedAt.Time,
			UpdatedAt:   request.UpdatedAt.Time,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode contact requests response", "err", err)
	}
}

func (h *contactsHandler) send(w http.ResponseWriter, r *http.Request) {
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

func (h *contactsHandler) cancel(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.service.cancel)
}

func (h *contactsHandler) accept(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.service.accept)
}

func (h *contactsHandler) decline(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, h.service.decline)
}

func (h *contactsHandler) mutate(w http.ResponseWriter, r *http.Request, action func(context.Context, string, pgtype.UUID) error) {
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

func (h *contactsHandler) writeError(w http.ResponseWriter, err error) {
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
