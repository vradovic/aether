package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vradovic/aether/backend/internal/core"
	"github.com/vradovic/aether/backend/internal/db"
)

var ErrUserNotFound = errors.New("user not found")
var ErrSelfRequest = errors.New("cannot send a contact request to yourself")
var ErrPendingRequestExists = errors.New("a pending contact request already exists")
var ErrRequestNotFound = errors.New("contact request not found")

type contactsService struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewContactsService(queries *db.Queries, pool *pgxpool.Pool) *contactsService {
	return &contactsService{queries: queries, pool: pool}
}

func (s *contactsService) Send(ctx context.Context, userID, username string) (pgtype.UUID, error) {
	senderID, err := core.ParseUUID(userID)
	if err != nil {
		return pgtype.UUID{}, err
	}

	request, err := s.queries.SendContactRequest(ctx, db.SendContactRequestParams{
		SenderID: senderID,
		Username: strings.TrimSpace(username),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, ErrUserNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23514":
			return pgtype.UUID{}, ErrSelfRequest
		case pgErr.ConstraintName == "contact_requests_one_pending_pair_idx":
			return pgtype.UUID{}, ErrPendingRequestExists
		}
	}
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("send contact request: %w", err)
	}
	return request.ID, nil
}

func (s *contactsService) GetPendingContactRequests(ctx context.Context, userID string) ([]db.ContactRequest, error) {
	recipientID, err := core.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	requests, err := s.queries.GetPendingContactRequests(ctx, recipientID)
	if err != nil {
		return nil, fmt.Errorf("get contact requests: %w", err)
	}
	return requests, nil
}

func (s *contactsService) Cancel(ctx context.Context, userID string, requestID pgtype.UUID) error {
	senderID, err := core.ParseUUID(userID)
	if err != nil {
		return err
	}
	_, err = s.queries.CancelContactRequest(ctx, db.CancelContactRequestParams{ID: requestID, SenderID: senderID})
	if err != nil {
		return fmt.Errorf("cancel contact request error: %w", err)
	}

	return nil
}

func (s *contactsService) Accept(ctx context.Context, userID string, requestID pgtype.UUID) error {
	recipientID, err := core.ParseUUID(userID)
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pool tx error: %w", err)
	}
	defer tx.Rollback(ctx)

	q := db.New(tx)
	req, err := q.AcceptContactRequest(ctx, db.AcceptContactRequestParams{ID: requestID, RecipientID: recipientID})
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

func (s *contactsService) Decline(ctx context.Context, userID string, requestID pgtype.UUID) error {
	recipientID, err := core.ParseUUID(userID)
	if err != nil {
		return err
	}

	if _, err = s.queries.DeclineContactRequest(ctx, db.DeclineContactRequestParams{ID: requestID, RecipientID: recipientID}); err != nil {
		return fmt.Errorf("decline contact request error: %w", err)
	}

	return nil
}
