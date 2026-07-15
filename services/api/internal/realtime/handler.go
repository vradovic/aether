package realtime

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/vradovic/aether/services/api/internal/core"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func ServeWs(w http.ResponseWriter, r *http.Request, logger *slog.Logger, manager *manager) {
	userID, ok := core.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "invalid user", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Warn("websocket upgrade failed", "error", err)
		return
	}

	// register conversations and connection
	c := &client{
		conn:    conn,
		send:    make(chan outboundMessage, 64),
		userID:  userID,
		manager: manager,
	}
	manager.register <- c

	// start pumps
	go c.readPump(logger)
	go c.writePump(logger)
}
