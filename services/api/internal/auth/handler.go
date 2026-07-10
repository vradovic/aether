package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

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
	mux.HandleFunc("POST /register", h.register)
}

func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB

	var dto registerRequest
	err := json.NewDecoder(r.Body).Decode(&dto)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err = h.svc.register(r.Context(), registerInput{
		email:     dto.Email,
		password:  dto.Password,
		firstName: dto.FirstName,
		lastName:  dto.LastName,
	})
	if err != nil {
		if errors.Is(err, errPasswordLength) ||
			errors.Is(err, errNameLength) ||
			errors.Is(err, errEmailFormat) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
