package realtime

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/vradovic/aether/backend/internal/core"
	"github.com/vradovic/aether/backend/internal/db"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func ServeWs(w http.ResponseWriter, r *http.Request, logger *slog.Logger, publisher publisher, router router, queries *db.Queries, secret string) {
	userID, err := ParseToken(w, r, secret)
	if err != nil {
		return
	}

	userUUID, err := core.ParseUUID(userID)
	if err != nil {
		logger.Warn("invalid user id uuid", "error", err)
		return
	}

	conversations, err := queries.GetConversationsForUser(r.Context(), userUUID)
	if err != nil {
		logger.Error("failed to get conversations for user", "error", err)
		return
	}

	convIDs := make([]string, 0, len(conversations))
	for _, conv := range conversations {
		convIDs = append(convIDs, conv.ID.String())
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Warn("websocket upgrade failed", "error", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	c := &client{
		conn:          conn,
		send:          make(chan outboundMessage, 64),
		userID:        userID,
		conversations: convIDs,
		logger:        logger,
		publisher:     publisher,
		router:        router,
	}

	router.register <- c

	go c.writePump()
	c.readPump(ctx)
}

func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func ParseToken(w http.ResponseWriter, r *http.Request, secret string) (userID string, err error) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "must include token", http.StatusBadRequest)
		return
	}

	userID, err = core.ParseTokenSubject(token, secret)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	return
}
