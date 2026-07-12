package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vradovic/aether/services/api/internal/auth"
	"github.com/vradovic/aether/services/api/internal/config"
	"github.com/vradovic/aether/services/api/internal/contacts"
	"github.com/vradovic/aether/services/api/internal/db"
	"github.com/vradovic/aether/services/api/internal/shared"
	"github.com/vradovic/aether/services/api/internal/users"
)

type server struct {
	cfg    *config.Config
	logger *slog.Logger
	mux    *http.ServeMux
	pool   *pgxpool.Pool
}

func NewServer(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*server, error) {
	mux := http.NewServeMux()

	pool, err := pgxpool.New(ctx, cfg.DbAddress)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	queries := db.New(pool)

	usersService := users.NewService(queries, logger)
	usersHandler := users.NewHandler(usersService, logger)

	tokenIssuer := shared.NewAccessTokenIssuer(
		cfg.JWTSigningKey,
		cfg.JWTIssuer,
		cfg.JWTAccessTokenTTL,
	)
	authService := auth.NewService(queries, tokenIssuer, logger)
	authHandler := auth.NewHandler(authService, logger)
	authMiddleware := shared.NewMiddleware(cfg.JWTSigningKey, cfg.JWTIssuer)

	contactsService := contacts.NewService(queries)
	contactsHandler := contacts.NewHandler(contactsService, logger)

	usersHandler.RegisterRoutes(mux)
	authHandler.RegisterRoutes(mux)
	contactsHandler.RegisterRoutes(mux, authMiddleware.Authenticate)

	return &server{
		cfg:    cfg,
		logger: logger,
		mux:    mux,
		pool:   pool,
	}, nil
}

func (s *server) Start() error {
	addr := s.cfg.ServerAddress

	s.logger.Info("starting server", "addr", addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second, // TODO: add timeouts to config
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return srv.ListenAndServe()
}

func (s *server) Close() {
	s.pool.Close()
}
