package users

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
)

type handler struct {
	svc    *service
	logger *slog.Logger
}

func NewHandler(svc *service, logger *slog.Logger) *handler {
	return &handler{
		svc:    svc,
		logger: logger,
	}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /users", h.getUserByEmail)
}

func (h *handler) getUserByEmail(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "email is empty", http.StatusBadRequest)
		return
	}

	row, err := h.svc.getUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := userResponse{
		ID:        row.ID.String(),
		Email:     row.Email,
		Username:  row.Username,
		FirstName: row.FirstName,
		LastName:  row.LastName,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", "err", err)
	}
}
