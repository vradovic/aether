package conversations

import (
	"context"
	"fmt"

	"github.com/gocql/gocql"
)

const (
	selectMessages      = `SELECT id, conversation_id, sender_id, client_message_id, body FROM messages WHERE conversation_id = ? LIMIT ?`
	selectMessagesAfter = `SELECT id, conversation_id, sender_id, client_message_id, body FROM messages WHERE conversation_id = ? AND id > ? LIMIT ?`
)

// ScyllaReader reads the message store the worker service writes to.
type ScyllaReader struct {
	session *gocql.Session
}

func NewScyllaReader(session *gocql.Session) *ScyllaReader {
	return &ScyllaReader{session: session}
}

func (sr *ScyllaReader) ReadMessages(ctx context.Context, conversationID, afterID gocql.UUID, pageSize int) ([]StoredMessage, error) {
	statement := selectMessages
	args := []any{conversationID, pageSize}
	if afterID != (gocql.UUID{}) {
		statement = selectMessagesAfter
		args = []any{conversationID, afterID, pageSize}
	}

	iter := sr.session.QueryWithContext(ctx, statement, args...).Iter()

	messages := make([]StoredMessage, 0, iter.NumRows())
	var message StoredMessage
	for iter.Scan(&message.ID, &message.ConversationID, &message.SenderID, &message.ClientMessageID, &message.Body) {
		messages = append(messages, message)
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("scan messages for conversation %s: %w", conversationID, err)
	}
	return messages, nil
}
