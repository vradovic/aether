package worker

import (
	"context"
	"encoding/json"
	"time"
)

type Message struct {
	ID              string    `json:"id"`
	SenderID        string    `json:"senderId"`
	ConversationID  string    `json:"conversationId"`
	ClientMessageID string    `json:"clientMessageId"`
	Body            string    `json:"body"`
	PublishedAt     time.Time `json:"publishedAt"`
}

type Writer interface {
	Write(context.Context, Message) (Message, error)
}

type Publisher interface {
	Publish(msg Message, data []byte) error
}

type Acker interface {
	Ack() error
	Nak() error
	Term() error
}

type Logger interface {
	Error(msg string, args ...any)
}

func Process(ctx context.Context, data []byte, writer Writer, publisher Publisher, acker Acker, logger Logger, metrics *Metrics) {
	msg, err := ParseMessage(data)
	if err != nil {
		logger.Error("message parse error", "error", err)
		TermLog(acker, logger)
		return
	}
	metrics.StageDuration.WithLabelValues("message_pull").Observe(time.Since(msg.PublishedAt).Seconds())
	processingStart := time.Now()

	msg, err = writer.Write(ctx, msg)
	if err != nil {
		logger.Error("message write error", "error", err)
		NakLog(acker, logger)
		return
	}

	data, err = json.Marshal(msg)
	if err != nil {
		logger.Error("message marshal error", "error", err)
		NakLog(acker, logger)
		return
	}

	msg.PublishedAt = time.Now()
	if err := publisher.Publish(msg, data); err != nil {
		logger.Error("message publish error", "error", err)
	}

	if err := acker.Ack(); err != nil {
		logger.Error("message ack error", "error", err)
		NakLog(acker, logger)
	}

	metrics.StageDuration.WithLabelValues("message_processing").Observe(time.Since(processingStart).Seconds())
}

func ParseMessage(data []byte) (msg Message, err error) {
	err = json.Unmarshal(data, &msg)
	return
}

func NakLog(acker Acker, logger Logger) {
	if err := acker.Nak(); err != nil {
		logger.Error("message nak error", "error", err)
	}
}

func TermLog(acker Acker, logger Logger) {
	if err := acker.Term(); err != nil {
		logger.Error("term error", "error", err)
	}
}
