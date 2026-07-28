package contacts

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/backend/internal/api"
	"github.com/vradovic/aether/backend/internal/api/httputil"
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
	Cancel(ctx context.Context, userID, requestID pgtype.UUID) error
	Accept(ctx context.Context, userID, requestID pgtype.UUID) error
	Decline(ctx context.Context, userID, requestID pgtype.UUID) error
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
		if errors.Is(err, core.ErrInvalidID) {
			httputil.Unauthorized(w)
			return
		}
		h.logger.Error("failed to parse user ID", "err", err)
		httputil.InternalServerError(w)
		return
	}

	requests, err := h.service.GetPendingContactRequests(r.Context(), recipientID)
	if err != nil {
		if errors.Is(err, core.ErrInvalidID) {
			httputil.Unauthorized(w)
			return
		}
		h.logger.Error("failed to get pending contact requests", "err", err)
		httputil.InternalServerError(w)
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

	httputil.WriteJSON(w, http.StatusOK, response, h.logger)
}

func (h *Handler) send(w http.ResponseWriter, r *http.Request, userID string) {
	var request sendRequest
	if err := httputil.DecodeJSON(w, r, &request); err != nil || request.Username == "" {
		httputil.BadRequest(w)
		return
	}

	senderID, err := core.ParseUUID(userID)
	if err != nil {
		if errors.Is(err, core.ErrInvalidID) {
			httputil.Unauthorized(w)
			return
		}
		h.logger.Error("failed to parse user ID", "err", err)
		httputil.InternalServerError(w)
		return
	}

	requestID, err := h.service.Send(r.Context(), senderID, request.Username)
	if err != nil {
		if errors.Is(err, core.ErrInvalidID) {
			httputil.Unauthorized(w)
			return
		}
		h.logger.Error("failed to send contact request", "err", err)
		httputil.InternalServerError(w)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, sendResponse{ID: requestID.String()}, h.logger)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request, userID string) {
	var requestID pgtype.UUID
	if err := requestID.Scan(r.PathValue("requestID")); err != nil || !requestID.Valid {
		httputil.BadRequest(w)
		return
	}

	senderID, err := core.ParseUUID(userID)
	if err != nil {
		if errors.Is(err, core.ErrInvalidID) {
			httputil.Unauthorized(w)
			return
		}
		h.logger.Error("failed to parse user ID", "err", err)
		httputil.InternalServerError(w)
		return
	}

	if err := h.service.Cancel(r.Context(), senderID, requestID); err != nil {
		if errors.Is(err, core.ErrInvalidID) {
			httputil.Unauthorized(w)
			return
		}
		h.logger.Error("failed to cancel contact request", "err", err)
		httputil.InternalServerError(w)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request, userID string) {
	var requestID pgtype.UUID
	if err := requestID.Scan(r.PathValue("requestID")); err != nil || !requestID.Valid {
		httputil.BadRequest(w)
		return
	}

	recipientID, err := core.ParseUUID(userID)
	if err != nil {
		if errors.Is(err, core.ErrInvalidID) {
			httputil.Unauthorized(w)
			return
		}
		h.logger.Error("failed to parse user ID", "err", err)
		httputil.InternalServerError(w)
		return
	}

	if err := h.service.Accept(r.Context(), recipientID, requestID); err != nil {
		if errors.Is(err, core.ErrInvalidID) {
			httputil.Unauthorized(w)
			return
		}
		h.logger.Error("failed to accept contact request", "err", err)
		httputil.InternalServerError(w)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) decline(w http.ResponseWriter, r *http.Request, userID string) {
	var requestID pgtype.UUID
	if err := requestID.Scan(r.PathValue("requestID")); err != nil || !requestID.Valid {
		httputil.BadRequest(w)
		return
	}

	recipientID, err := core.ParseUUID(userID)
	if err != nil {
		if errors.Is(err, core.ErrInvalidID) {
			httputil.Unauthorized(w)
			return
		}
		h.logger.Error("failed to parse user ID", "err", err)
		httputil.InternalServerError(w)
		return
	}

	if err := h.service.Decline(r.Context(), recipientID, requestID); err != nil {
		if errors.Is(err, core.ErrInvalidID) {
			httputil.Unauthorized(w)
			return
		}
		h.logger.Error("failed to decline contact request", "err", err)
		httputil.InternalServerError(w)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
