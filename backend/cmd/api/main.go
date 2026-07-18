package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vradovic/aether/services/api/internal/api"
	"github.com/vradovic/aether/services/api/internal/db"
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

	middleware := api.Middleware{SigningKey: cfg.JWTSigningKey}

	authService := api.NewAuthService(queries, cfg.JWTSigningKey)
	authHandler := api.NewAuthHandler(authService, logger)

	contactsService := api.NewContactsService(queries)
	contactsHandler := api.NewContactsHandler(contactsService, logger)

	conversationsService := api.NewConversationsService(queries, logger)
	conversationsHandler := api.NewConversationsHandler(conversationsService, logger)

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
