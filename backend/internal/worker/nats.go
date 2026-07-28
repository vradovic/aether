package worker

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

type NatsPublisher struct {
	nc *nats.Conn
}

func NewNatsPublisher(nc *nats.Conn) *NatsPublisher {
	return &NatsPublisher{nc: nc}
}

func (np *NatsPublisher) Publish(msg Message, data []byte) error {
	if err := np.nc.Publish(fmt.Sprintf("messages.processed.%s", msg.ConversationID), data); err != nil {
		return err
	}

	return nil
}

type NatsAcker struct {
	msg *nats.Msg
}

func NewNatsAcker(msg *nats.Msg) *NatsAcker {
	return &NatsAcker{msg: msg}
}

func (na *NatsAcker) Ack() error {
	return na.msg.Ack()
}

func (na *NatsAcker) Nak() error {
	return na.msg.Nak()
}

func (na *NatsAcker) Term() error {
	return na.msg.Term()
}
