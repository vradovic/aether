package contacts

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/db"
)

const testUserID = "550e8400-e29b-41d4-a716-446655440000"

var testRequestID = pgtype.UUID{
	Bytes: [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x01},
	Valid: true,
}

type fakeQuerier struct {
	sendParams    db.SendContactRequestParams
	sendID        pgtype.UUID
	sendErr       error
	cancelParams  db.CancelContactRequestParams
	cancelErr     error
	acceptParams  db.AcceptContactRequestParams
	acceptErr     error
	declineParams db.DeclineContactRequestParams
	declineErr    error
}

func (f *fakeQuerier) SendContactRequest(_ context.Context, params db.SendContactRequestParams) (db.ContactRequest, error) {
	f.sendParams = params
	return db.ContactRequest{ID: f.sendID}, f.sendErr
}

func (f *fakeQuerier) CancelContactRequest(_ context.Context, params db.CancelContactRequestParams) (db.ContactRequest, error) {
	f.cancelParams = params
	return db.ContactRequest{ID: params.ID}, f.cancelErr
}

func (f *fakeQuerier) AcceptContactRequest(_ context.Context, params db.AcceptContactRequestParams) (db.ContactRequest, error) {
	f.acceptParams = params
	return db.ContactRequest{ID: params.ID}, f.acceptErr
}

func (f *fakeQuerier) DeclineContactRequest(_ context.Context, params db.DeclineContactRequestParams) (db.ContactRequest, error) {
	f.declineParams = params
	return db.ContactRequest{ID: params.ID}, f.declineErr
}

func TestServiceSend(t *testing.T) {
	t.Run("trims username and sends request as authenticated user", func(t *testing.T) {
		queries := &fakeQuerier{sendID: testRequestID}
		requestID, err := NewService(queries).send(context.Background(), testUserID, "  petar ")
		if err != nil {
			t.Fatalf("send() error = %v", err)
		}
		if requestID != testRequestID {
			t.Errorf("send() ID = %v, want %v", requestID, testRequestID)
		}
		if queries.sendParams.Username != "petar" {
			t.Errorf("username = %q, want %q", queries.sendParams.Username, "petar")
		}
		if queries.sendParams.SenderID.String() != testUserID {
			t.Errorf("sender ID = %q, want %q", queries.sendParams.SenderID.String(), testUserID)
		}
	})

	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "unknown username", err: pgx.ErrNoRows, want: ErrUserNotFound},
		{name: "self request", err: &pgconn.PgError{Code: "23514"}, want: ErrSelfRequest},
		{name: "duplicate pending request", err: &pgconn.PgError{Code: "23505", ConstraintName: "contact_requests_one_pending_pair_idx"}, want: ErrPendingRequestExists},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(&fakeQuerier{sendErr: tt.err}).send(context.Background(), testUserID, "petar")
			if !errors.Is(err, tt.want) {
				t.Fatalf("send() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestServiceMutations(t *testing.T) {
	t.Run("cancel uses sender identity", func(t *testing.T) {
		queries := &fakeQuerier{}
		if err := NewService(queries).cancel(context.Background(), testUserID, testRequestID); err != nil {
			t.Fatalf("cancel() error = %v", err)
		}
		if queries.cancelParams.SenderID.String() != testUserID || queries.cancelParams.ID != testRequestID {
			t.Errorf("cancel params = %#v", queries.cancelParams)
		}
	})

	t.Run("accept uses recipient identity", func(t *testing.T) {
		queries := &fakeQuerier{}
		if err := NewService(queries).accept(context.Background(), testUserID, testRequestID); err != nil {
			t.Fatalf("accept() error = %v", err)
		}
		if queries.acceptParams.RecipientID.String() != testUserID || queries.acceptParams.ID != testRequestID {
			t.Errorf("accept params = %#v", queries.acceptParams)
		}
	})

	t.Run("decline uses recipient identity", func(t *testing.T) {
		queries := &fakeQuerier{}
		if err := NewService(queries).decline(context.Background(), testUserID, testRequestID); err != nil {
			t.Fatalf("decline() error = %v", err)
		}
		if queries.declineParams.RecipientID.String() != testUserID || queries.declineParams.ID != testRequestID {
			t.Errorf("decline params = %#v", queries.declineParams)
		}
	})

	t.Run("missing or unauthorized request is not found", func(t *testing.T) {
		queries := &fakeQuerier{cancelErr: pgx.ErrNoRows, acceptErr: pgx.ErrNoRows, declineErr: pgx.ErrNoRows}
		service := NewService(queries)
		if err := service.cancel(context.Background(), testUserID, testRequestID); !errors.Is(err, ErrRequestNotFound) {
			t.Errorf("cancel() error = %v, want %v", err, ErrRequestNotFound)
		}
		if err := service.accept(context.Background(), testUserID, testRequestID); !errors.Is(err, ErrRequestNotFound) {
			t.Errorf("accept() error = %v, want %v", err, ErrRequestNotFound)
		}
		if err := service.decline(context.Background(), testUserID, testRequestID); !errors.Is(err, ErrRequestNotFound) {
			t.Errorf("decline() error = %v, want %v", err, ErrRequestNotFound)
		}
	})
}
