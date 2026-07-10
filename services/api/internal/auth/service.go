package auth

import (
	"context"
	"log/slog"
	"strings"

	"github.com/vradovic/aether/services/api/internal/db"
)

type queries interface {
	CreateUser(ctx context.Context, arg db.CreateUserParams) error
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

func (s *service) register(ctx context.Context, dto registerRequest) error {
	if err := dto.validate(); err != nil {
		return err
	}

	passwordHash, err := hashPassword(dto.Password)
	if err != nil {
		return err
	}

	email := strings.TrimSpace(dto.Email)
	email = strings.ToLower(email)

	firstName := strings.TrimSpace(dto.FirstName)
	lastName := strings.TrimSpace(dto.LastName)

	return s.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        email,
		PasswordHash: passwordHash,
		FirstName:    firstName,
		LastName:     lastName,
	})
}
