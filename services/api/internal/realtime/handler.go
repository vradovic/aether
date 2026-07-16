package realtime

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/vradovic/aether/services/api/internal/core"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func ServeWs(w http.ResponseWriter, r *http.Request, logger *slog.Logger, publisher publisher, router router, secret string) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "must include token", http.StatusBadRequest)
		return
	}

	userID, err := core.ParseTokenSubject(token, secret)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Warn("websocket upgrade failed", "error", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	c := &client{
		conn:      conn,
		send:      make(chan outboundMessage, 64),
		userID:    userID,
		logger:    logger,
		publisher: publisher,
		router:    router,
	}

	router.register <- c

	go c.writePump()
	c.readPump(ctx)
}
