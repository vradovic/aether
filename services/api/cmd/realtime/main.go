package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/vradovic/aether/services/api/internal/db"
	"github.com/vradovic/aether/services/api/internal/realtime"
	"github.com/vradovic/aether/services/api/internal/shared"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("realtime service stopped", "error", err)
		os.Exit(1)
	}
}

type componentResult struct {
	name string
	err  error
}

func run(logger *slog.Logger) error {
	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	cfg, err := realtime.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	pool, err := pgxpool.New(ctx, cfg.DbAddress)
	if err != nil {
		return fmt.Errorf("create pgx pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping pgx: %w", err)
	}

	queries := db.New(pool)

	nc, err := nats.Connect(cfg.NatsAddress)
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	defer nc.Close()

	manager := realtime.NewManager(ctx, nc, pool, queries, logger)
	authMiddleware := shared.NewMiddleware(cfg.JWTSigningKey, cfg.JWTIssuer)
	mux := http.NewServeMux()

	wsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realtime.ServeWs(w, r, logger, manager)
	})
	mux.Handle("/ws", authMiddleware.Authenticate(wsHandler))

	server := &http.Server{
		Addr:              cfg.ServerAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	results := make(chan componentResult, 2)
	go func() {
		results <- componentResult{name: "manager", err: manager.Run()}
	}()
	go func() {
		results <- componentResult{name: "HTTP server", err: server.ListenAndServe()}
	}()

	completed := 0
	var runErr error

	select {
	case <-ctx.Done():
		// SIGINT or SIGTERM initiated a normal shutdown.
	case result := <-results:
		completed++
		runErr = componentError(result, ctx)
	}

	// Stop the manager.
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := server.Shutdown(shutdownCtx); err != nil {
		runErr = errors.Join(runErr, fmt.Errorf("shut down HTTP server: %w", err))
	}
	shutdownCancel()

	for completed < 2 {
		result := <-results
		completed++
		if err := componentError(result, ctx); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}

	return runErr
}

func componentError(result componentResult, ctx context.Context) error {
	if ctx.Err() != nil && (result.err == nil || errors.Is(result.err, context.Canceled) || errors.Is(result.err, http.ErrServerClosed)) {
		return nil
	}
	if result.err == nil {
		return fmt.Errorf("%s stopped unexpectedly", result.name)
	}
	return fmt.Errorf("%s stopped unexpectedly: %w", result.name, result.err)
}
