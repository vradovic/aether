package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

type router struct {
	ctx    context.Context
	logger *slog.Logger

	register   chan *client
	unregister chan *client

	registry map[string]map[*client]struct{} // registry of clients, user id maps to map of clients

	nc      *nats.Conn
	subject string
}

func NewRouter(ctx context.Context, logger *slog.Logger, nc *nats.Conn, subject string) router {
	return router{
		ctx:        ctx,
		logger:     logger,
		nc:         nc,
		subject:    subject,
		register:   make(chan *client),
		unregister: make(chan *client),
		registry:   make(map[string]map[*client]struct{}),
	}
}

func (r router) Run() error {
	natsCh := make(chan *nats.Msg, 100)

	sub, err := r.nc.ChanSubscribe(r.subject, natsCh)
	if err != nil {
		return fmt.Errorf("subscribe to %q: %w", r.subject, err)
	}
	defer func() {
		sub.Unsubscribe()
	}()

	for {
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case c := <-r.register:
			r.registerClient(c)
		case c := <-r.unregister:
			r.unregisterClient(c)
		case natsMsg := <-natsCh:
			var msg eventMessage
			err := json.Unmarshal(natsMsg.Data, &msg)
			if err != nil {
				r.logger.Error("failed to unmarshal data", "error", err)
				continue
			}

			r.deliver(msg)
		}
	}
}

func (r router) registerClient(c *client) {
	if r.registry[c.userID] == nil {
		r.registry[c.userID] = make(map[*client]struct{})
	}

	r.registry[c.userID][c] = struct{}{}
}

func (r router) unregisterClient(c *client) {
	clients, ok := r.registry[c.userID]
	if !ok {
		return
	}

	if _, ok := clients[c]; !ok {
		return
	}

	delete(clients, c)
	close(c.send)

	if len(clients) == 0 {
		delete(r.registry, c.userID)
	}
}

func (r router) deliver(msg eventMessage) {
	for _, id := range msg.Recipients {
		if clients, ok := r.registry[id]; ok {
			for c := range clients {
				select {
				case c.send <- msg.outboundMessage:
				default: // if client is slow
					r.logger.Warn("disconnecting slow client", "userID", c.userID, "conn", c.conn)
					r.unregisterClient(c)
				}
			}
		}
	}
}
