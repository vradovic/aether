package users

import (
	"context"
	"log/slog"

	"github.com/vradovic/aether/services/api/internal/db"
)

type queries interface {
	GetUserByEmail(ctx context.Context, email string) (db.GetUserByEmailRow, error)
}

type service struct {
	queries queries
	logger  *slog.Logger
}

func NewService(queries queries, logger *slog.Logger) *service {
	return &service{
		queries: queries,
		logger:  logger,
	}
}

func (s *service) getUserByEmail(ctx context.Context, email string) (db.GetUserByEmailRow, error) {
	return s.queries.GetUserByEmail(ctx, email)
}
