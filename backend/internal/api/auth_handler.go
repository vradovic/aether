package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

type RegisterRequest struct {
	Email     string `json:"email"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"accessToken"`
	TokenType   string `json:"tokenType"`
	ExpiresIn   int64  `json:"expiresIn"`
}

type AuthService interface {
	Login(ctx context.Context, input LoginInput) (LoginOutput, error)
	Register(ctx context.Context, input RegisterInput) error
}

type authHandler struct {
	svc    AuthService
	logger *slog.Logger
}

func NewAuthHandler(svc AuthService, logger *slog.Logger) *authHandler {
	return &authHandler{
		svc:    svc,
		logger: logger,
	}
}

func (h *authHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /register", h.Register)
	mux.HandleFunc("POST /login", h.Login)
}

func (h *authHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB

	var request LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	output, err := h.svc.Login(r.Context(), LoginInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		h.logger.Error("login failed", "err", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(LoginResponse{
		AccessToken: output.AccessToken,
		TokenType:   "Bearer",
	}); err != nil {
		h.logger.Error("failed to encode login response", "err", err)
	}
}

func (h *authHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB

	var dto RegisterRequest
	err := json.NewDecoder(r.Body).Decode(&dto)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err = h.svc.Register(r.Context(), RegisterInput{
		email:     dto.Email,
		username:  dto.Username,
		password:  dto.Password,
		firstName: dto.FirstName,
		lastName:  dto.LastName,
	})
	if err != nil {
		if errors.Is(err, errPasswordLength) ||
			errors.Is(err, errNameLength) ||
			errors.Is(err, errUsernameLength) ||
			errors.Is(err, errEmailFormat) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
