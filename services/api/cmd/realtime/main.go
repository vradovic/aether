package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/vradovic/aether/services/api/internal/db"
	"github.com/vradovic/aether/services/api/internal/realtime"
	"github.com/vradovic/aether/services/api/internal/shared"
)

func main() {
	ctx := context.Background()

	cfg, err := realtime.LoadConfig()
	if err != nil {
		log.Fatal("config load error: ", err)
	}

	pool, err := pgxpool.New(ctx, cfg.DbAddress)
	if err != nil {
		log.Fatal("pgx pool create fail: ", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		log.Fatal("ping to pgx fail: ", err)
	}

	queries := db.New(pool)

	nc, err := nats.Connect(cfg.NatsAddress)
	if err != nil {
		log.Fatal("nats connection error: ", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	defer func() {
		nc.Close()
		pool.Close()
	}()

	manager := realtime.NewManager(ctx, nc, pool, queries, logger)
	managerErrCh := make(chan error)
	go func() {
		managerErrCh <- manager.Run()
	}()
	authMiddleware := shared.NewMiddleware(cfg.JWTSigningKey, cfg.JWTIssuer)
	mux := http.NewServeMux()

	wsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realtime.ServeWs(w, r, logger, manager)
	})
	mux.Handle("/ws", authMiddleware.Authenticate(wsHandler))

	if err := http.ListenAndServe(cfg.ServerAddress, mux); err != nil {
		log.Fatal("http server error: ", err)
	}
}
