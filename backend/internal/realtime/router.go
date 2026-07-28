package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

type convSubscription struct {
	sub      *nats.Subscription
	refCount int
}

type router struct {
	ctx                 context.Context
	logger              *slog.Logger
	register            chan *client
	unregister          chan *client
	registry            map[string]map[*client]struct{} // user id maps to map of clients
	conversationClients map[string]map[*client]struct{} // conversation id maps to map of clients
	subscriptions       map[string]*convSubscription    // conversation id maps to convSubscription
	natsCh              chan *nats.Msg
	nc                  *nats.Conn
	subject             string
}

func NewRouter(ctx context.Context, logger *slog.Logger, nc *nats.Conn, subject string) router {
	return router{
		ctx:                 ctx,
		logger:              logger,
		nc:                  nc,
		subject:             subject,
		register:            make(chan *client),
		unregister:          make(chan *client),
		registry:            make(map[string]map[*client]struct{}),
		conversationClients: make(map[string]map[*client]struct{}),
		subscriptions:       make(map[string]*convSubscription),
		natsCh:              make(chan *nats.Msg, 100),
	}
}

func (r router) Run() error {
	defer func() {
		for convID, subEntry := range r.subscriptions {
			subEntry.sub.Unsubscribe()
			delete(r.subscriptions, convID)
		}
	}()

	for {
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case c := <-r.register:
			r.registerClient(c)
		case c := <-r.unregister:
			r.unregisterClient(c)
		case natsMsg := <-r.natsCh:
			var msg outboundMessage
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

	for _, convID := range c.conversations {
		if r.conversationClients[convID] == nil {
			r.conversationClients[convID] = make(map[*client]struct{})
		}
		r.conversationClients[convID][c] = struct{}{}

		if subEntry, ok := r.subscriptions[convID]; ok {
			subEntry.refCount++
		} else {
			subject := fmt.Sprintf("messages.processed.%s", convID)
			sub, err := r.nc.ChanSubscribe(subject, r.natsCh)
			if err != nil {
				r.logger.Error("failed to subscribe to conversation topic", "subject", subject, "error", err)
				continue
			}
			r.subscriptions[convID] = &convSubscription{
				sub:      sub,
				refCount: 1,
			}
			r.logger.Debug("subscribed to conversation topic", "subject", subject)
		}
	}
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

	for _, convID := range c.conversations {
		if convClients, ok := r.conversationClients[convID]; ok {
			delete(convClients, c)
			if len(convClients) == 0 {
				delete(r.conversationClients, convID)
			}
		}

		if subEntry, ok := r.subscriptions[convID]; ok {
			subEntry.refCount--
			if subEntry.refCount <= 0 {
				if err := subEntry.sub.Unsubscribe(); err != nil {
					r.logger.Error("failed to unsubscribe from conversation topic", "convID", convID, "error", err)
				}
				delete(r.subscriptions, convID)
				r.logger.Debug("unsubscribed from conversation topic", "convID", convID)
			}
		}
	}
}

func (r router) deliver(msg outboundMessage) {
	clients, ok := r.conversationClients[msg.ConversationID]
	if !ok {
		return
	}

	for c := range clients {
		select {
		case c.send <- msg:
		default: // if client is slow
			r.logger.Warn("disconnecting slow client", "userID", c.userID, "conn", c.conn)
			r.unregisterClient(c)
		}
	}
}
