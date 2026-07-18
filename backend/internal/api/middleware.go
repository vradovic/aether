package api

import (
	"net/http"

	"github.com/vradovic/aether/services/api/internal/core"
)

type Middleware struct {
	SigningKey string
}

type HandlerWithUser func(w http.ResponseWriter, r *http.Request, userID string)

func (m Middleware) RequireAuth(next HandlerWithUser) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		tokenString, err := core.ExtractBearerToken(authorization)
		if err != nil {
			httpUnauthorized(w)
			return
		}

		id, err := core.ParseTokenSubject(tokenString, m.SigningKey)
		if err != nil {
			httpUnauthorized(w)
			return
		}

		next(w, r, id)
	})
}
