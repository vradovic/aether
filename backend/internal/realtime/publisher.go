package realtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

type publisher struct {
	nc      *nats.Conn
	subject string
}

func NewPublisher(nc *nats.Conn, subject string) publisher {
	return publisher{
		nc:      nc,
		subject: subject,
	}
}

func (p publisher) publish(ctx context.Context, msg publishMessage) error {
	bytes, err := json.Marshal(&msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err = p.nc.Publish(p.subject, bytes); err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}

	return nil
}
