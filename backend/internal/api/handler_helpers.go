package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func httpInternalServerError(w http.ResponseWriter) {
	http.Error(w, "Unexpected error occurred.", http.StatusInternalServerError)
}

func httpUnauthorized(w http.ResponseWriter) {
	http.Error(w, "User is unauthorized", http.StatusUnauthorized)
}

func writeJSONResponse(w http.ResponseWriter, status int, v any, logger *slog.Logger) {
	data, err := json.Marshal(v)
	if err != nil {
		httpInternalServerError(w)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		logger.Error("failed to write bytes", "error", err)
	}
}
