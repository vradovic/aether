package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
)

type userResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type usersHandler struct {
	svc    *usersService
	logger *slog.Logger
}

func NewUsersHandler(svc *usersService, logger *slog.Logger) *usersHandler {
	return &usersHandler{
		svc:    svc,
		logger: logger,
	}
}

func (h *usersHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /users", h.getUserByEmail)
}

func (h *usersHandler) getUserByEmail(w http.ResponseWriter, r *http.Request) {
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
