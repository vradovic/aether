package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

const conversationChannelBuffer = 32

type convSubscription struct {
	sub      *nats.Subscription
	natsCh   chan *nats.Msg
	refCount int
}

type Router struct {
	ctx                 context.Context
	logger              *slog.Logger
	register            chan *client
	unregister          chan *client
	mu                  *sync.RWMutex
	registry            map[string]map[*client]struct{} // user id maps to map of clients
	conversationClients map[string]map[*client]struct{} // conversation id maps to map of clients
	subscriptions       map[string]*convSubscription    // conversation id maps to convSubscription
	nc                  *nats.Conn
	subject             string
	metrics             *Metrics
}

func NewRouter(ctx context.Context, logger *slog.Logger, nc *nats.Conn, subject string, metrics *Metrics) Router {
	return Router{
		ctx:                 ctx,
		logger:              logger,
		nc:                  nc,
		subject:             subject,
		register:            make(chan *client),
		unregister:          make(chan *client),
		mu:                  &sync.RWMutex{},
		registry:            make(map[string]map[*client]struct{}),
		conversationClients: make(map[string]map[*client]struct{}),
		subscriptions:       make(map[string]*convSubscription),
		metrics:             metrics,
	}
}

func (r Router) Run() error {
	defer func() {
		for convID, subEntry := range r.subscriptions {
			subEntry.sub.Unsubscribe()
			close(subEntry.natsCh)
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
		}
	}
}

// processConversation is launched in its own goroutine for each conversation
// that has at least one subscriber. It owns natsCh exclusively and exits
// once natsCh is closed on unsubscribe.
func (r Router) processConversation(natsCh chan *nats.Msg) {
	for natsMsg := range natsCh {
		var msg outboundMessage
		if err := json.Unmarshal(natsMsg.Data, &msg); err != nil {
			r.logger.Error("failed to unmarshal data", "error", err)
			continue
		}

		r.metrics.StageDuration.WithLabelValues("processed_message_pull").Observe(time.Since(msg.PublishedAt).Seconds())
		tm := TraceMessage{}
		tm.outboundMessage = msg
		tm.TraceTime = time.Now()

		r.deliver(tm)
	}
}

// NatsChannelDepth reports the combined backlog across all per-conversation
// NATS channels, for the depth gauge polled from outside the router.
func (r Router) NatsChannelDepth() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	depth := 0
	for _, subEntry := range r.subscriptions {
		depth += len(subEntry.natsCh)
	}
	return depth
}

func (r Router) registerClient(c *client) {
	if r.registry[c.userID] == nil {
		r.registry[c.userID] = make(map[*client]struct{})
	}
	r.registry[c.userID][c] = struct{}{}

	for _, convID := range c.conversations {
		r.mu.Lock()
		if r.conversationClients[convID] == nil {
			r.conversationClients[convID] = make(map[*client]struct{})
		}
		r.conversationClients[convID][c] = struct{}{}

		if subEntry, ok := r.subscriptions[convID]; ok {
			subEntry.refCount++
			r.mu.Unlock()
			continue
		}
		r.mu.Unlock()

		subject := fmt.Sprintf("messages.processed.%s", convID)
		natsCh := make(chan *nats.Msg, conversationChannelBuffer)
		sub, err := r.nc.ChanSubscribe(subject, natsCh)
		if err != nil {
			r.logger.Error("failed to subscribe to conversation topic", "subject", subject, "error", err)
			continue
		}

		r.mu.Lock()
		r.subscriptions[convID] = &convSubscription{
			sub:      sub,
			natsCh:   natsCh,
			refCount: 1,
		}
		r.mu.Unlock()

		go r.processConversation(natsCh)
		r.logger.Debug("subscribed to conversation topic", "subject", subject)
	}
}

func (r Router) unregisterClient(c *client) {
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
		r.mu.Lock()
		if convClients, ok := r.conversationClients[convID]; ok {
			delete(convClients, c)
			if len(convClients) == 0 {
				delete(r.conversationClients, convID)
			}
		}

		subEntry, ok := r.subscriptions[convID]
		if !ok {
			r.mu.Unlock()
			continue
		}

		subEntry.refCount--
		if subEntry.refCount > 0 {
			r.mu.Unlock()
			continue
		}

		delete(r.subscriptions, convID)
		r.mu.Unlock()

		if err := subEntry.sub.Unsubscribe(); err != nil {
			r.logger.Error("failed to unsubscribe from conversation topic", "convID", convID, "error", err)
		}
		close(subEntry.natsCh)
		r.logger.Debug("unsubscribed from conversation topic", "convID", convID)
	}
}

func (r Router) deliver(msg TraceMessage) {
	r.mu.RLock()
	clients, ok := r.conversationClients[msg.ConversationID]
	if !ok {
		r.mu.RUnlock()
		return
	}

	targets := make([]*client, 0, len(clients))
	for c := range clients {
		targets = append(targets, c)
	}
	r.mu.RUnlock()

	for _, c := range targets {
		select {
		case c.send <- msg:
		default: // if client is slow
			r.logger.Warn("disconnecting slow client", "userID", c.userID, "conn", c.conn)
			r.unregister <- c
		}
	}
}
