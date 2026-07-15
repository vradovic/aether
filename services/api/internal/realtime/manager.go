package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/vradovic/aether/services/api/internal/db"
	"github.com/vradovic/aether/services/api/internal/core"
)

const (
	subject = "message.created"
)

type manager struct {
	publish    chan publishMessage
	unregister chan *client
	register   chan *client

	clients map[string]*client

	nc *nats.Conn

	pool    *pgxpool.Pool
	queries *db.Queries

	ctx context.Context

	logger *slog.Logger
}

func NewManager(ctx context.Context, nc *nats.Conn, pool *pgxpool.Pool, queries *db.Queries, logger *slog.Logger) *manager {
	return &manager{
		publish:    make(chan publishMessage),
		unregister: make(chan *client),
		register:   make(chan *client),
		clients:    make(map[string]*client),
		nc:         nc,
		pool:       pool,
		queries:    queries,
		ctx:        ctx,
		logger:     logger,
	}
}

func (m *manager) Run() error {
	msgCh := make(chan *nats.Msg, 64)

	sub, err := m.nc.ChanSubscribe(subject, msgCh)
	if err != nil {
		return fmt.Errorf("subscribe to %q: %w", subject, err)
	}
	defer func() {
		sub.Unsubscribe()
	}()

	for {
		select {
		// TODO: Current implementation does not enable multiple client websocket connections for same user id, leave for now since only one client app ...
		case c := <-m.register:
			if current, ok := m.clients[c.userID]; ok {
				close(current.send)
			}
			m.clients[c.userID] = c
		case c := <-m.unregister:
			if current, ok := m.clients[c.userID]; !ok || current != c { // if old client sends unregister
				continue
			}
			delete(m.clients, c.userID)
			close(c.send)
		// MESSAGE READING
		case natsMsg := <-msgCh:
			var msg eventMessage
			err := json.Unmarshal(natsMsg.Data, &msg)
			if err != nil {
				m.logger.Warn("failed to unmarshal data", "error", err)
				continue
			}

			for _, id := range msg.Recipients {
				if c, ok := m.clients[id]; ok {
					select {
					case c.send <- msg.outboundMessage:
					default:
						delete(m.clients, id)
						close(c.send)
					}
				}
			}
		// MESSAGE PUBLISHING
		case msg := <-m.publish:
			// needs to validate conversation participation and set and increment message sequence
			ok, err := m.validateParticipant(msg.ConversationID, msg.SenderID)
			if err != nil {
				m.logger.Warn("validation failed", "error", err)
				continue
			}
			if !ok { // not in convo
				m.logger.Debug("not in conversation", "userID", msg.SenderID)
				continue
			}

			ids, err := m.getRecipients(msg.ConversationID)
			if err != nil {
				m.logger.Warn("failed to fetch recipients", "error", err)
				continue
			}
			msg.Recipients = ids

			// NOTE: If process fails after insertMessage and before Publish, the db will contain the message but clients wont receive it
			publishMsg, err := m.insertMessage(msg)
			if err != nil {
				m.logger.Warn("failed to insert message", "error", err)
				continue
			}

			bytes, err := json.Marshal(&publishMsg)
			if err != nil {
				m.logger.Warn("failed to marshal message", "error", err)
				continue
			}

			if err = m.nc.Publish(subject, bytes); err != nil {
				m.logger.Warn("failed to publish message", "error", err)
			}
		case <-m.ctx.Done():
			for _, c := range m.clients {
				delete(m.clients, c.userID)
				close(c.send)
			}
			return m.ctx.Err()
		}
	}
}

func (m *manager) validateParticipant(conversationIDString, userIDString string) (bool, error) {
	conversationID, err := core.ParseUUID(conversationIDString)
	if err != nil {
		return false, err
	}

	userID, err := core.ParseUUID(userIDString)
	if err != nil {
		return false, err
	}

	ok, err := m.queries.IsConversationParticipant(m.ctx, db.IsConversationParticipantParams{
		ConversationID: conversationID,
		UserID:         userID,
	})
	if err != nil {
		return false, err
	}

	return ok, nil
}

func (m *manager) getRecipients(conversationIDString string) ([]string, error) {
	conversationID, err := core.ParseUUID(conversationIDString)
	if err != nil {
		return nil, err
	}

	ids, err := m.queries.GetConversationRecipientIDs(m.ctx, conversationID)
	if err != nil {
		return nil, err
	}

	stringIds := make([]string, 0, len(ids))
	for _, id := range ids {
		stringIds = append(stringIds, id.String())
	}

	return stringIds, nil
}

func (m *manager) insertMessage(msg publishMessage) (publishMessage, error) {
	conversationID, err := core.ParseUUID(msg.ConversationID)
	if err != nil {
		return publishMessage{}, err
	}

	senderID, err := core.ParseUUID(msg.SenderID)
	if err != nil {
		return publishMessage{}, err
	}

	clientMessageID, err := core.ParseUUID(msg.ClientMessageID)
	if err != nil {
		return publishMessage{}, err
	}

	tx, err := m.pool.Begin(m.ctx)
	if err != nil {
		return publishMessage{}, err
	}
	defer tx.Rollback(m.ctx)

	txQuerier := m.queries.WithTx(tx)

	sequence, err := txQuerier.NextMessageSequence(m.ctx, conversationID)
	if err != nil {
		return publishMessage{}, err
	}

	row, err := txQuerier.InsertMessage(m.ctx, db.InsertMessageParams{
		ConversationID:  conversationID,
		MessageSequence: sequence,
		SenderID:        senderID,
		Body:            msg.Body,
		ClientMessageID: clientMessageID,
	})
	if err != nil {
		return publishMessage{}, err
	}

	if err := tx.Commit(m.ctx); err != nil {
		return publishMessage{}, err
	}

	return publishMessage{
		ID: row.ID.String(),
		inboundMessage: inboundMessage{
			ConversationID:  row.ConversationID.String(),
			ClientMessageID: row.ClientMessageID.String(),
			Body:            row.Body,
		},
		SenderID:        row.SenderID.String(),
		MessageSequence: row.MessageSequence,
		Recipients:      msg.Recipients,
	}, nil
}
