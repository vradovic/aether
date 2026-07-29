package contacts

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vradovic/aether/backend/internal/db"
)

type Service struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewService(queries *db.Queries, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, pool: pool}
}

func (s *Service) Send(ctx context.Context, userID pgtype.UUID, username string) (pgtype.UUID, error) {
	request, err := s.queries.SendContactRequest(ctx, db.SendContactRequestParams{
		SenderID: userID,
		Username: strings.TrimSpace(username),
	})
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("send contact request: %w", err)
	}
	return request.ID, nil
}

func (s *Service) GetPendingContactRequests(ctx context.Context, userID pgtype.UUID) ([]db.ContactRequest, error) {
	requests, err := s.queries.GetPendingContactRequests(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get contact requests: %w", err)
	}
	return requests, nil
}

func (s *Service) Cancel(ctx context.Context, userID pgtype.UUID, requestID pgtype.UUID) error {
	_, err := s.queries.CancelContactRequest(ctx, db.CancelContactRequestParams{ID: requestID, SenderID: userID})
	if err != nil {
		return fmt.Errorf("cancel contact request error: %w", err)
	}
	return nil
}

func (s *Service) Accept(ctx context.Context, userID pgtype.UUID, requestID pgtype.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pool tx error: %w", err)
	}
	defer tx.Rollback(ctx)

	q := db.New(tx)
	req, err := q.AcceptContactRequest(ctx, db.AcceptContactRequestParams{ID: requestID, RecipientID: userID})
	if err != nil {
		return fmt.Errorf("accept contact request error: %w", err)
	}

	if _, err = q.InsertContact(ctx, db.InsertContactParams{Column1: req.RecipientID, Column2: req.SenderID}); err != nil {
		return fmt.Errorf("insert contact error: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("tx commit error: %w", err)
	}

	return nil
}

func (s *Service) Decline(ctx context.Context, userID pgtype.UUID, requestID pgtype.UUID) error {
	if _, err := s.queries.DeclineContactRequest(ctx, db.DeclineContactRequestParams{ID: requestID, RecipientID: userID}); err != nil {
		return fmt.Errorf("decline contact request error: %w", err)
	}
	return nil
}
