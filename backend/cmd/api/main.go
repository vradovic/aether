package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vradovic/aether/backend/internal/api"
	"github.com/vradovic/aether/backend/internal/api/auth"
	"github.com/vradovic/aether/backend/internal/api/contacts"
	"github.com/vradovic/aether/backend/internal/api/conversations"
	"github.com/vradovic/aether/backend/internal/core"
	"github.com/vradovic/aether/backend/internal/db"
)

func main() {
	cfg, err := api.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DbAddress)
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("failed to ping pool: %v", err)
	}

	queries := db.New(pool)

	scyllaHosts := strings.Split(cfg.ScyllaHosts, ",")
	session, err := core.NewScyllaCluster(scyllaHosts, cfg.ScyllaKeyspace).CreateSession()
	if err != nil {
		log.Fatalf("failed to connect to scylladb: %v", err)
	}
	defer session.Close()
	logger.Info("connected to scylladb", "hosts", scyllaHosts, "keyspace", cfg.ScyllaKeyspace)

	middleware := api.Middleware{SigningKey: cfg.JWTSigningKey}

	authService := auth.NewService(queries, cfg.JWTSigningKey)
	authHandler := auth.NewHandler(authService, logger)

	contactsService := contacts.NewService(queries, pool)
	contactsHandler := contacts.NewHandler(contactsService, logger)

	conversationsService := conversations.NewService(queries, conversations.NewScyllaReader(session), logger)
	conversationsHandler := conversations.NewHandler(conversationsService, logger)

	mux := http.NewServeMux()
	authHandler.RegisterRoutes(mux)
	contactsHandler.RegisterRoutes(mux, middleware)
	conversationsHandler.RegisterRoutes(mux, middleware)

	server := http.Server{
		Addr:              cfg.ServerAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // TODO: add timeouts to config
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("starting server", "address", cfg.ServerAddress)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
