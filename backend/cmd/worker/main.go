package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gocql/gocql"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	js, err := jetstream.New(nc)
	if err != nil {
		logger.Error("failed to get jetstream context", "error", err)
		os.Exit(1)
	}

	ctxNats, cancelNats := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelNats()

	stream, err := js.Stream(ctxNats, cfg.NatsStream)
	if err != nil {
		logger.Error("failed to get stream", "error", err)
		os.Exit(1)
	}

	cons, err := stream.Consumer(ctxNats, cfg.NatsDurable)
	if err != nil {
		logger.Error("failed to get consumer", "error", err)
		os.Exit(1)
	}

	registry, metrics := worker.InitMetrics()

	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: mux,
	}

	errCh := make(chan error)

	go func() {
		errCh <- server.ListenAndServe()
	}()

	logger.Info("starting worker consumer")

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		acker := worker.NewNatsAcker(msg)
		worker.Process(ctx, msg.Data(), writer, publisher, acker, logger, metrics)
	})
	defer cc.Stop()
	if err != nil {
		logger.Error("failed to start consuming", "error", err)
		os.Exit(1)
	}

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("service failure", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutting down worker")
		return
	}
}
