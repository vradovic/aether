package api

import (
	"context"
	"log/slog"

	"github.com/vradovic/aether/services/api/internal/db"
)

type usersQuerier interface {
	GetUserByEmail(ctx context.Context, email string) (db.GetUserByEmailRow, error)
}

type usersService struct {
	queries usersQuerier
	logger  *slog.Logger
}

func NewUsersService(queries usersQuerier, logger *slog.Logger) *usersService {
	return &usersService{
		queries: queries,
		logger:  logger,
	}
}

func (s *usersService) getUserByEmail(ctx context.Context, email string) (db.GetUserByEmailRow, error) {
	return s.queries.GetUserByEmail(ctx, email)
}
