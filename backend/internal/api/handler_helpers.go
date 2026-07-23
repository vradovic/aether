package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

func httpInternalServerError(w http.ResponseWriter) {
	http.Error(w, "Unexpected error occurred.", http.StatusInternalServerError)
}

func httpUnauthorized(w http.ResponseWriter) {
	http.Error(w, "User is unauthorized", http.StatusUnauthorized)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain one JSON value")
		}
		return err
	}
	return nil
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
