package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vradovic/aether/services/api/internal/core"
	"github.com/vradovic/aether/services/api/internal/db"
)

type server struct {
	cfg    *Config
	logger *slog.Logger
	mux    *http.ServeMux
	pool   *pgxpool.Pool
}

func NewServer(ctx context.Context, cfg *Config, logger *slog.Logger) (*server, error) {
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

	usersService := NewUsersService(queries, logger)
	usersHandler := NewUsersHandler(usersService, logger)

	tokenIssuer := core.NewAccessTokenIssuer(
		cfg.JWTSigningKey,
		cfg.JWTIssuer,
		cfg.JWTAccessTokenTTL,
	)
	authService := NewAuthService(queries, tokenIssuer, logger)
	authHandler := NewAuthHandler(authService, logger)
	authMiddleware := core.NewMiddleware(cfg.JWTSigningKey, cfg.JWTIssuer)

	contactsService := NewContactsService(queries)
	contactsHandler := NewContactsHandler(contactsService, logger)

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
