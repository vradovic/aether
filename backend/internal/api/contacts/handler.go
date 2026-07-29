package contacts

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/backend/internal/api"
	"github.com/vradovic/aether/backend/internal/core"
	"github.com/vradovic/aether/backend/internal/db"
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

type ServiceInterface interface {
	Send(ctx context.Context, userID pgtype.UUID, username string) (pgtype.UUID, error)
	GetPendingContactRequests(ctx context.Context, userID pgtype.UUID) ([]db.ContactRequest, error)
	Cancel(ctx context.Context, userID string, requestID pgtype.UUID) error
	Accept(ctx context.Context, userID string, requestID pgtype.UUID) error
	Decline(ctx context.Context, userID string, requestID pgtype.UUID) error
}

type Handler struct {
	service ServiceInterface
	logger  *slog.Logger
}

func NewHandler(service ServiceInterface, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, m api.Middleware) {
	mux.Handle("POST /contact-requests", m.RequireAuth(h.send))
	mux.Handle("GET /contact-requests", m.RequireAuth(h.getPendingContactRequests))
	mux.Handle("PATCH /contact-requests/{requestID}/cancel", m.RequireAuth(h.cancel))
	mux.Handle("PATCH /contact-requests/{requestID}/accept", m.RequireAuth(h.accept))
	mux.Handle("PATCH /contact-requests/{requestID}/decline", m.RequireAuth(h.decline))
}

func (h *Handler) getPendingContactRequests(w http.ResponseWriter, r *http.Request, userID string) {
	recipientID, err := core.ParseUUID(userID)
	if err != nil {
		h.writeError(w, err)
		return
	}

	requests, err := h.service.GetPendingContactRequests(r.Context(), recipientID)
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

func (h *Handler) send(w http.ResponseWriter, r *http.Request, userID string) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var request sendRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Username == "" {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	senderID, err := core.ParseUUID(userID)
	if err != nil {
		h.writeError(w, err)
		return
	}

	requestID, err := h.service.Send(r.Context(), senderID, request.Username)
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

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request, userID string) {
	var requestID pgtype.UUID
	if err := requestID.Scan(r.PathValue("requestID")); err != nil || !requestID.Valid {
		http.Error(w, "invalid contact request ID", http.StatusBadRequest)
		return
	}
	if err := h.service.Cancel(r.Context(), userID, requestID); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request, userID string) {
	var requestID pgtype.UUID
	if err := requestID.Scan(r.PathValue("requestID")); err != nil || !requestID.Valid {
		http.Error(w, "invalid contact request ID", http.StatusBadRequest)
		return
	}
	if err := h.service.Accept(r.Context(), userID, requestID); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) decline(w http.ResponseWriter, r *http.Request, userID string) {
	var requestID pgtype.UUID
	if err := requestID.Scan(r.PathValue("requestID")); err != nil || !requestID.Valid {
		http.Error(w, "invalid contact request ID", http.StatusBadRequest)
		return
	}
	if err := h.service.Decline(r.Context(), userID, requestID); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
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
