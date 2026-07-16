package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/core"
	"github.com/vradovic/aether/services/api/internal/db"
)

var ErrUserNotFound = errors.New("user not found")
var ErrSelfRequest = errors.New("cannot send a contact request to yourself")
var ErrPendingRequestExists = errors.New("a pending contact request already exists")
var ErrRequestNotFound = errors.New("contact request not found")

type contactsQuerier interface {
	SendContactRequest(context.Context, db.SendContactRequestParams) (db.ContactRequest, error)
	CancelContactRequest(context.Context, db.CancelContactRequestParams) (db.ContactRequest, error)
	AcceptContactRequest(context.Context, db.AcceptContactRequestParams) (db.ContactRequest, error)
	DeclineContactRequest(context.Context, db.DeclineContactRequestParams) (db.ContactRequest, error)
}

type contactsService struct {
	queries contactsQuerier
}

func NewContactsService(queries contactsQuerier) *contactsService {
	return &contactsService{queries: queries}
}

func (s *contactsService) send(ctx context.Context, userID, username string) (pgtype.UUID, error) {
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

func (s *contactsService) cancel(ctx context.Context, userID string, requestID pgtype.UUID) error {
	senderID, err := core.ParseUUID(userID)
	if err != nil {
		return err
	}
	_, err = s.queries.CancelContactRequest(ctx, db.CancelContactRequestParams{ID: requestID, SenderID: senderID})
	return mapRequestMutationError("cancel contact request", err)
}

func (s *contactsService) accept(ctx context.Context, userID string, requestID pgtype.UUID) error {
	recipientID, err := core.ParseUUID(userID)
	if err != nil {
		return err
	}
	_, err = s.queries.AcceptContactRequest(ctx, db.AcceptContactRequestParams{ID: requestID, RecipientID: recipientID})
	return mapRequestMutationError("accept contact request", err)
}

func (s *contactsService) decline(ctx context.Context, userID string, requestID pgtype.UUID) error {
	recipientID, err := core.ParseUUID(userID)
	if err != nil {
		return err
	}
	_, err = s.queries.DeclineContactRequest(ctx, db.DeclineContactRequestParams{ID: requestID, RecipientID: recipientID})
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
