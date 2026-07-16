package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/vradovic/aether/services/api/internal/db"
	"github.com/vradovic/aether/services/api/internal/realtime"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := realtime.LoadConfig()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	logger.Info("loaded config")

	pool, err := pgxpool.New(context.Background(), cfg.DbAddress)
	if err != nil {
		logger.Error("create pgx pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("created pg pool")

	if err := pool.Ping(context.Background()); err != nil {
		logger.Error("ping pgx", "error", err)
		os.Exit(1)
	}

	queries := db.New(pool)

	nc, err := nats.Connect(cfg.NatsAddress)
	if err != nil {
		logger.Error("connect to nats", "error", err)
		os.Exit(1)
	}
	defer nc.Close()
	logger.Info("connected to nats")

	errCh := make(chan error, 2)

	router := realtime.NewRouter(context.Background(), logger, nc, cfg.NATSSubject)
	go func() {
		errCh <- router.Run()
	}()

	publisher := realtime.NewPublisher(nc, pool, queries, cfg.NATSSubject)

	mux := http.NewServeMux()

	wsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realtime.ServeWs(w, r, logger, publisher, router, cfg.JWTSigningKey)
	})
	mux.Handle("/ws", wsHandler)

	server := &http.Server{
		Addr:              cfg.ServerAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		errCh <- server.ListenAndServe()
	}()

	if err := <-errCh; err != nil {
		logger.Error("service failure", "error", err)
		os.Exit(1)
	}
}
