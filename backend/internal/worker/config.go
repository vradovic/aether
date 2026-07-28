package worker

import (
	"fmt"
	"os"
)

type Config struct {
	ScyllaHosts    string
	ScyllaKeyspace string
	NatsAddress    string
	NatsStream     string
	NatsDurable    string
	NatsSubject    string
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		ScyllaHosts:    os.Getenv("SCYLLA_HOSTS"),
		ScyllaKeyspace: os.Getenv("SCYLLA_KEYSPACE"),
		NatsAddress:    os.Getenv("NATS_ADDRESS"),
		NatsStream:     os.Getenv("NATS_STREAM"),
		NatsDurable:    os.Getenv("NATS_DURABLE"),
		NatsSubject:    os.Getenv("NATS_SUBJECT"),
	}

	if cfg.ScyllaHosts == "" {
		return nil, fmt.Errorf("SCYLLA_HOSTS is required")
	}

	if cfg.ScyllaKeyspace == "" {
		return nil, fmt.Errorf("SCYLLA_KEYSPACE is required")
	}

	if cfg.NatsAddress == "" {
		return nil, fmt.Errorf("NATS_ADDRESS is required")
	}

	if cfg.NatsStream == "" {
		return nil, fmt.Errorf("NATS_STREAM is required")
	}

	if cfg.NatsDurable == "" {
		return nil, fmt.Errorf("NATS_DURABLE is required")
	}

	return cfg, nil
}
