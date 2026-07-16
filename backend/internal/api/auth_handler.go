package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

type registerRequest struct {
	Email     string `json:"email"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"accessToken"`
	TokenType   string `json:"tokenType"`
	ExpiresIn   int64  `json:"expiresIn"`
}

type authHandler struct {
	svc    *authService
	logger *slog.Logger
}

func NewAuthHandler(svc *authService, logger *slog.Logger) *authHandler {
	return &authHandler{
		svc:    svc,
		logger: logger,
	}
}

func (h *authHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /register", h.register)
	mux.HandleFunc("POST /login", h.login)
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB

	var request loginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	output, err := h.svc.login(r.Context(), loginInput{
		email:    request.Email,
		password: request.Password,
	})
	if err != nil {
		if errors.Is(err, errInvalidCredentials) {
			http.Error(w, errInvalidCredentials.Error(), http.StatusUnauthorized)
			return
		}

		h.logger.Error("login failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(loginResponse{
		AccessToken: output.accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   output.expiresInSeconds,
	}); err != nil {
		h.logger.Error("failed to encode login response", "err", err)
	}
}

func (h *authHandler) register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB

	var dto registerRequest
	err := json.NewDecoder(r.Body).Decode(&dto)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err = h.svc.register(r.Context(), registerInput{
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

	w.WriteHeader(http.StatusCreated)
}
