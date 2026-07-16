package realtime

import (
	"context"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

// Reference https://github.com/gorilla/websocket/blob/main/examples/chat/client.go
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
)

type client struct {
	logger    *slog.Logger
	userID    string
	conn      *websocket.Conn
	send      chan outboundMessage
	publisher publisher
	router    router
}

func (c *client) readPump(ctx context.Context) {
	defer func() {
		c.router.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		var msg inboundMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			c.logger.Warn("read message fail", "error", err)
			return
		}

		if err := c.publisher.publish(ctx, publishMessage{
			inboundMessage: msg,
			SenderID:       c.userID,
			// MessageSequence gets set by publisher
		}); err != nil {

		}
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteJSON(msg)
			if err != nil {
				c.logger.Warn("websocket write failed", "error", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
