package realtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/vradovic/aether/services/api/internal/core"
	"github.com/vradovic/aether/services/api/internal/db"
)

type publisher struct {
	nc      *nats.Conn
	pool    *pgxpool.Pool
	queries *db.Queries
	subject string
}

func NewPublisher(nc *nats.Conn, pool *pgxpool.Pool, queries *db.Queries, subject string) publisher {
	return publisher{
		nc:      nc,
		pool:    pool,
		queries: queries,
		subject: subject,
	}
}

// Performs validation, db inserts and publishes to nats
func (p publisher) publish(ctx context.Context, msg publishMessage) error {
	ids, err := p.getRecipients(ctx, msg.ConversationID)
	if err != nil {
		return fmt.Errorf("getRecipients: %w", err)
	}
	msg.Recipients = ids

	// NOTE: If process fails after insertMessage and before Publish, the db will contain the message but clients won't receive it
	publishMsg, err := p.insertMessage(ctx, msg)
	if err != nil {
		return fmt.Errorf("insertMessage: %w", err)
	}

	bytes, err := json.Marshal(&publishMsg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err = p.nc.Publish(p.subject, bytes); err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}

	return nil
}

func (p publisher) getRecipients(ctx context.Context, conversationIDString string) ([]string, error) {
	conversationID, err := core.ParseUUID(conversationIDString)
	if err != nil {
		return nil, err
	}

	ids, err := p.queries.GetConversationRecipientIDs(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	stringIds := make([]string, 0, len(ids))
	for _, id := range ids {
		stringIds = append(stringIds, id.String())
	}

	return stringIds, nil
}

func (p publisher) insertMessage(ctx context.Context, msg publishMessage) (publishMessage, error) {
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

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return publishMessage{}, err
	}
	defer tx.Rollback(ctx)

	txQuerier := p.queries.WithTx(tx)

	sequence, err := txQuerier.NextMessageSequence(ctx, conversationID)
	if err != nil {
		return publishMessage{}, err
	}

	row, err := txQuerier.InsertMessage(ctx, db.InsertMessageParams{
		ConversationID:  conversationID,
		MessageSequence: sequence,
		SenderID:        senderID,
		Body:            msg.Body,
		ClientMessageID: clientMessageID,
	})
	if err != nil {
		return publishMessage{}, err
	}

	if err := tx.Commit(ctx); err != nil {
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
