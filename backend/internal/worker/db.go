package worker

import (
	"context"

	"github.com/gocql/gocql"
)

type ScyllaWriter struct {
	session *gocql.Session
}

func NewScyllaWriter(session *gocql.Session) *ScyllaWriter {
	return &ScyllaWriter{session: session}
}

func (sw *ScyllaWriter) Write(ctx context.Context, msg Message) (Message, error) {
	id := gocql.TimeUUID()
	if err := sw.session.QueryWithContext(
		ctx,
		`INSERT INTO messages (id, sender_id, conversation_id, client_message_id, body) VALUES (?, ?, ?, ?, ?)`,
		id, msg.SenderID, msg.ConversationID, msg.ClientMessageID, msg.Body,
	).Exec(); err != nil {
		return Message{}, err
	}

	msg.ID = id.String()

	return msg, nil
}
