package realtime

import (
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
	logger        *slog.Logger
	userID        string
	conversations []string
	conn          *websocket.Conn
	send          chan TraceMessage
	publisher     publisher
	router        Router
	metrics       *Metrics
}

func (c *client) readPump() {
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

		c.logger.Debug("read message", "msg", msg)
		publishedAt := time.Now()
		if err := c.publisher.publish(publishMessage{
			inboundMessage: msg,
			SenderID:       c.userID,
			PublishedAt:    publishedAt,
		}); err != nil {
			c.logger.Warn("publish message fail", "error", err)
		}

		c.metrics.StageDuration.WithLabelValues("unprocessed_message_publish").Observe(time.Since(publishedAt).Seconds())
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

			err := c.conn.WriteJSON(msg.outboundMessage)
			if err != nil {
				c.logger.Warn("websocket write failed", "error", err)
				return
			}
			c.metrics.StageDuration.WithLabelValues("message_write_to_client").Observe(time.Since(msg.TraceTime).Seconds())
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
