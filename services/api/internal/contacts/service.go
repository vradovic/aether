package contacts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/db"
)

var ErrUserNotFound = errors.New("user not found")
var ErrSelfRequest = errors.New("cannot send a contact request to yourself")
var ErrPendingRequestExists = errors.New("a pending contact request already exists")
var ErrRequestNotFound = errors.New("contact request not found")
var ErrInvalidUserID = errors.New("invalid authenticated user ID")

type querier interface {
	SendContactRequest(context.Context, db.SendContactRequestParams) (pgtype.UUID, error)
	CancelContactRequest(context.Context, db.CancelContactRequestParams) (pgtype.UUID, error)
	AcceptContactRequest(context.Context, db.AcceptContactRequestParams) (bool, error)
	DeclineContactRequest(context.Context, db.DeclineContactRequestParams) (pgtype.UUID, error)
}

type service struct {
	queries querier
}

func NewService(queries querier) *service {
	return &service{queries: queries}
}

func parseUserID(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return pgtype.UUID{}, ErrInvalidUserID
	}
	return id, nil
}

func (s *service) send(ctx context.Context, userID, username string) (pgtype.UUID, error) {
	senderID, err := parseUserID(userID)
	if err != nil {
		return pgtype.UUID{}, err
	}

	requestID, err := s.queries.SendContactRequest(ctx, db.SendContactRequestParams{
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
	return requestID, nil
}

func (s *service) cancel(ctx context.Context, userID string, requestID pgtype.UUID) error {
	senderID, err := parseUserID(userID)
	if err != nil {
		return err
	}
	_, err = s.queries.CancelContactRequest(ctx, db.CancelContactRequestParams{RequestID: requestID, SenderID: senderID})
	return mapRequestMutationError("cancel contact request", err)
}

func (s *service) accept(ctx context.Context, userID string, requestID pgtype.UUID) error {
	recipientID, err := parseUserID(userID)
	if err != nil {
		return err
	}
	accepted, err := s.queries.AcceptContactRequest(ctx, db.AcceptContactRequestParams{RequestID: requestID, RecipientID: recipientID})
	if err != nil {
		return fmt.Errorf("accept contact request: %w", err)
	}
	if !accepted {
		return ErrRequestNotFound
	}
	return nil
}

func (s *service) decline(ctx context.Context, userID string, requestID pgtype.UUID) error {
	recipientID, err := parseUserID(userID)
	if err != nil {
		return err
	}
	_, err = s.queries.DeclineContactRequest(ctx, db.DeclineContactRequestParams{RequestID: requestID, RecipientID: recipientID})
	return mapRequestMutationError("decline contact request", err)
}

func mapRequestMutationError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRequestNotFound
	}
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}
