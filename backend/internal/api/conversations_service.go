package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/db"
)

var (
	ErrNotFound = errors.New("no conversations found")
)

type Conversation struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	CreatedBy           string `json:"createdBy"`
	LastMessageSequence int64  `json:"lastMessageSequence"`
}

type conversationsQuerier interface {
	GetConversationsForUser(ctx context.Context, userID pgtype.UUID) ([]db.Conversation, error)
}

type conversationsService struct {
	querier conversationsQuerier
	logger  *slog.Logger
}

func NewConversationsService(querier conversationsQuerier, logger *slog.Logger) *conversationsService {
	return &conversationsService{
		querier: querier,
		logger:  logger,
	}
}

func (s *conversationsService) getConversations(ctx context.Context, userID pgtype.UUID) ([]Conversation, error) {
	convosDb, err := s.querier.GetConversationsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get conversations for userID %s: %w", userID, err)
	}

	convos := make([]Conversation, 0, len(convosDb))
	for _, v := range convosDb {
		convos = append(convos, Conversation{
			ID:                  v.ID.String(),
			Name:                userID.String(),
			CreatedBy:           v.CreatedBy.String(),
			LastMessageSequence: v.LastMessageSequence,
		})
	}

	return convos, err
}
