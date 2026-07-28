package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gocql/gocql"
	"github.com/nats-io/nats.go"
	"github.com/vradovic/aether/backend/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := worker.LoadConfig()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	logger.Info("loaded config")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	scyllaHosts := strings.Split(cfg.ScyllaHosts, ",")

	cluster := gocql.NewCluster(scyllaHosts...)
	cluster.Keyspace = cfg.ScyllaKeyspace
	cluster.Consistency = gocql.Quorum

	if len(scyllaHosts) == 1 && (scyllaHosts[0] == "127.0.0.1" || scyllaHosts[0] == "localhost") {
		cluster.AddressTranslator = gocql.AddressTranslatorFunc(func(ip net.IP, port int) (net.IP, int) {
			return net.ParseIP(scyllaHosts[0]), port
		})
	}

	session, err := cluster.CreateSession()
	if err != nil {
		logger.Error("failed to connect to scylladb", "error", err, "hosts", scyllaHosts, "keyspace", cfg.ScyllaKeyspace)
		os.Exit(1)
	}
	defer session.Close()
	logger.Info("connected to scylladb", "hosts", scyllaHosts, "keyspace", cfg.ScyllaKeyspace)

	writer := worker.NewScyllaWriter(session)

	nc, err := nats.Connect(cfg.NatsAddress)
	if err != nil {
		logger.Error("failed to connect to nats", "error", err, "address", cfg.NatsAddress)
		os.Exit(1)
	}
	defer nc.Close()
	logger.Info("connected to nats", "address", cfg.NatsAddress)

	publisher := worker.NewNatsPublisher(nc)

	js, err := nc.JetStream()
	if err != nil {
		logger.Error("failed to get jetstream context", "error", err)
		os.Exit(1)
	}

	var subOpts []nats.SubOpt
	if cfg.NatsStream != "" {
		subOpts = append(subOpts, nats.BindStream(cfg.NatsStream))
	}

	sub, err := js.PullSubscribe(cfg.NatsSubject, cfg.NatsDurable, subOpts...)
	if err != nil {
		logger.Error("failed to subscribe to nats durable consumer", "error", err, "stream", cfg.NatsStream, "durable", cfg.NatsDurable, "subject", cfg.NatsSubject)
		os.Exit(1)
	}
	defer sub.Unsubscribe()
	logger.Info("subscribed to nats durable consumer", "stream", cfg.NatsStream, "durable", cfg.NatsDurable, "subject", cfg.NatsSubject)

	logger.Info("starting worker processing loop")

	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down worker")
			return
		default:
		}

		msgs, err := sub.Fetch(10, nats.Context(ctx))
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, nats.ErrTimeout) {
				continue
			}
			logger.Error("error fetching messages from nats stream", "error", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		for _, msg := range msgs {
			acker := worker.NewNatsAcker(msg)
			worker.Process(ctx, msg.Data, writer, publisher, acker, logger)
		}
	}
}
