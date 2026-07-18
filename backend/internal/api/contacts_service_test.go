package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/core"
	"github.com/vradovic/aether/services/api/internal/db"
)

func TestContactsService(t *testing.T) {
	ctx := context.Background()
	conn := startContactsTestDatabase(t, ctx)
	service := NewContactsService(db.New(conn))

	t.Run("send", func(t *testing.T) {
		t.Run("creates a pending request for a trimmed username", func(t *testing.T) {
			senderID := createContactsTestUser(t, ctx, conn, "send_sender")
			recipientID := createContactsTestUser(t, ctx, conn, "send_recipient")

			requestID, err := service.Send(ctx, senderID.String(), "  send_recipient  ")
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}
			if !requestID.Valid {
				t.Fatal("Send() returned an invalid request ID")
			}

			var gotSenderID, gotRecipientID pgtype.UUID
			var status string
			err = conn.QueryRow(ctx, `
				SELECT sender_id, recipient_id, status
				FROM contact_requests
				WHERE id = $1`, requestID).Scan(&gotSenderID, &gotRecipientID, &status)
			if err != nil {
				t.Fatalf("query contact request: %v", err)
			}
			if gotSenderID != senderID || gotRecipientID != recipientID || status != "pending" {
				t.Fatalf("stored request = sender %s, recipient %s, status %q",
					gotSenderID.String(), gotRecipientID.String(), status)
			}
		})

		t.Run("returns user not found for an unknown username", func(t *testing.T) {
			senderID := createContactsTestUser(t, ctx, conn, "missing_sender")

			requestID, err := service.Send(ctx, senderID.String(), "does_not_exist")
			if !errors.Is(err, ErrUserNotFound) {
				t.Fatalf("Send() error = %v, want %v", err, ErrUserNotFound)
			}
			if requestID.Valid {
				t.Fatalf("Send() request ID = %s, want invalid UUID", requestID.String())
			}
		})

		t.Run("maps the distinct-users check constraint", func(t *testing.T) {
			userID := createContactsTestUser(t, ctx, conn, "self_request")

			_, err := service.Send(ctx, userID.String(), "self_request")
			if !errors.Is(err, ErrSelfRequest) {
				t.Fatalf("Send() error = %v, want %v", err, ErrSelfRequest)
			}
		})

		t.Run("maps the pending-pair unique index in both directions", func(t *testing.T) {
			user1ID := createContactsTestUser(t, ctx, conn, "duplicate_user_1")
			user2ID := createContactsTestUser(t, ctx, conn, "duplicate_user_2")
			if _, err := service.Send(ctx, user1ID.String(), "duplicate_user_2"); err != nil {
				t.Fatalf("seed Send() error = %v", err)
			}

			for _, tt := range []struct {
				name     string
				senderID pgtype.UUID
				username string
			}{
				{name: "same direction", senderID: user1ID, username: "duplicate_user_2"},
				{name: "reverse direction", senderID: user2ID, username: "duplicate_user_1"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					_, err := service.Send(ctx, tt.senderID.String(), tt.username)
					if !errors.Is(err, ErrPendingRequestExists) {
						t.Fatalf("Send() error = %v, want %v", err, ErrPendingRequestExists)
					}
				})
			}
		})

		t.Run("returns the existing-contacts trigger error with context", func(t *testing.T) {
			user1ID := createContactsTestUser(t, ctx, conn, "contact_user_1")
			user2ID := createContactsTestUser(t, ctx, conn, "contact_user_2")
			if _, err := conn.Exec(ctx, `
				INSERT INTO contacts (user1_id, user2_id)
				VALUES (LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid))`, user1ID, user2ID); err != nil {
				t.Fatalf("insert contact: %v", err)
			}

			_, err := service.Send(ctx, user1ID.String(), "contact_user_2")
			if err == nil || !strings.Contains(err.Error(), "send contact request") {
				t.Fatalf("Send() error = %v, want wrapped operation context", err)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "P0001" || pgErr.Message != "users are already contacts" {
				t.Fatalf("Send() error = %v, want existing-contacts trigger error", err)
			}
		})

		t.Run("wraps a sender foreign-key violation", func(t *testing.T) {
			createContactsTestUser(t, ctx, conn, "fk_recipient")
			missingSenderID := parseContactsTestUUID(t, "10000000-0000-0000-0000-000000000001")

			_, err := service.Send(ctx, missingSenderID.String(), "fk_recipient")
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23503" ||
				pgErr.ConstraintName != "contact_requests_sender_id_fkey" {
				t.Fatalf("Send() error = %v, want sender foreign-key violation", err)
			}
			if !strings.Contains(err.Error(), "send contact request") {
				t.Fatalf("Send() error = %v, want operation context", err)
			}
		})
	})

	t.Run("get pending contact requests", func(t *testing.T) {
		recipientID := createContactsTestUser(t, ctx, conn, "pending_recipient")
		olderSenderID := createContactsTestUser(t, ctx, conn, "pending_older_sender")
		newerSenderID := createContactsTestUser(t, ctx, conn, "pending_newer_sender")
		outgoingRecipientID := createContactsTestUser(t, ctx, conn, "pending_outgoing_recipient")
		declinedSenderID := createContactsTestUser(t, ctx, conn, "pending_declined_sender")

		olderID := insertContactsTestRequest(t, ctx, conn, olderSenderID, recipientID, "pending", time.Now().Add(-time.Hour))
		newerID := insertContactsTestRequest(t, ctx, conn, newerSenderID, recipientID, "pending", time.Now())
		insertContactsTestRequest(t, ctx, conn, recipientID, outgoingRecipientID, "pending", time.Now().Add(time.Hour))
		insertContactsTestRequest(t, ctx, conn, declinedSenderID, recipientID, "declined", time.Now().Add(2*time.Hour))

		requests, err := service.GetPendingContactRequests(ctx, recipientID.String())
		if err != nil {
			t.Fatalf("GetPendingContactRequests() error = %v", err)
		}
		if len(requests) != 2 {
			t.Fatalf("GetPendingContactRequests() returned %d requests, want 2", len(requests))
		}
		if requests[0].ID != newerID || requests[1].ID != olderID {
			t.Fatalf("request order = [%s, %s], want [%s, %s]",
				requests[0].ID.String(), requests[1].ID.String(), newerID.String(), olderID.String())
		}
		for _, request := range requests {
			if request.RecipientID != recipientID || request.Status != "pending" {
				t.Fatalf("unexpected request returned: %+v", request)
			}
		}
	})

	t.Run("mutate contact request", func(t *testing.T) {
		tests := []struct {
			name       string
			wantStatus string
			wrongActor func(senderID, recipientID pgtype.UUID) pgtype.UUID
			mutate     func(context.Context, string, pgtype.UUID) error
		}{
			{
				name:       "cancel",
				wantStatus: "cancelled",
				wrongActor: func(_, recipientID pgtype.UUID) pgtype.UUID { return recipientID },
				mutate:     service.Cancel,
			},
			{
				name:       "accept",
				wantStatus: "accepted",
				wrongActor: func(senderID, _ pgtype.UUID) pgtype.UUID { return senderID },
				mutate:     service.Accept,
			},
			{
				name:       "decline",
				wantStatus: "declined",
				wrongActor: func(senderID, _ pgtype.UUID) pgtype.UUID { return senderID },
				mutate:     service.Decline,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				senderID := createContactsTestUser(t, ctx, conn, tt.name+"_sender")
				recipientID := createContactsTestUser(t, ctx, conn, tt.name+"_recipient")
				requestID, err := service.Send(ctx, senderID.String(), tt.name+"_recipient")
				if err != nil {
					t.Fatalf("seed Send() error = %v", err)
				}

				err = tt.mutate(ctx, tt.wrongActor(senderID, recipientID).String(), requestID)
				if !errors.Is(err, ErrRequestNotFound) {
					t.Fatalf("mutation by wrong actor error = %v, want %v", err, ErrRequestNotFound)
				}

				actorID := senderID
				if tt.name != "cancel" {
					actorID = recipientID
				}
				if err := tt.mutate(ctx, actorID.String(), requestID); err != nil {
					t.Fatalf("mutation error = %v", err)
				}

				var status string
				if err := conn.QueryRow(ctx, "SELECT status FROM contact_requests WHERE id = $1", requestID).Scan(&status); err != nil {
					t.Fatalf("query request status: %v", err)
				}
				if status != tt.wantStatus {
					t.Fatalf("request status = %q, want %q", status, tt.wantStatus)
				}

				if err := tt.mutate(ctx, actorID.String(), requestID); !errors.Is(err, ErrRequestNotFound) {
					t.Fatalf("repeated mutation error = %v, want %v", err, ErrRequestNotFound)
				}
			})
		}
	})

	t.Run("rejects invalid user IDs", func(t *testing.T) {
		requestID := parseContactsTestUUID(t, "20000000-0000-0000-0000-000000000001")
		tests := []struct {
			name string
			call func() error
		}{
			{
				name: "send",
				call: func() error {
					_, err := service.Send(ctx, "not-a-uuid", "someone")
					return err
				},
			},
			{
				name: "get pending",
				call: func() error {
					_, err := service.GetPendingContactRequests(ctx, "not-a-uuid")
					return err
				},
			},
			{name: "cancel", call: func() error { return service.Cancel(ctx, "not-a-uuid", requestID) }},
			{name: "accept", call: func() error { return service.Accept(ctx, "not-a-uuid", requestID) }},
			{name: "decline", call: func() error { return service.Decline(ctx, "not-a-uuid", requestID) }},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if err := tt.call(); !errors.Is(err, core.ErrInvalidID) {
					t.Fatalf("error = %v, want %v", err, core.ErrInvalidID)
				}
			})
		}
	})
}

func startContactsTestDatabase(t *testing.T, ctx context.Context) *pgx.Conn {
	t.Helper()

	conn := startAuthTestDatabase(t, ctx)
	for _, migrationName := range []string{
		"20260710211453_create_contacts.sql",
		"20260710214332_add_contact_requests.sql",
	} {
		migrationPath := filepath.Join("..", "..", "sql", "migrations", migrationName)
		migration, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatalf("read migration %s: %v", migrationName, err)
		}
		upMigration, _, found := strings.Cut(string(migration), "-- +goose Down")
		if !found {
			t.Fatalf("migration %s has no Goose down marker", migrationName)
		}
		if _, err := conn.Exec(ctx, upMigration); err != nil {
			t.Fatalf("apply migration %s: %v", migrationName, err)
		}
	}
	return conn
}

func createContactsTestUser(t *testing.T, ctx context.Context, conn *pgx.Conn, username string) pgtype.UUID {
	t.Helper()

	var id pgtype.UUID
	err := conn.QueryRow(ctx, `
		INSERT INTO users (email, username, password_hash, first_name, last_name)
		VALUES ($1, $2, 'unused-password-hash', 'Test', 'User')
		RETURNING id`, fmt.Sprintf("%s@example.com", username), username).Scan(&id)
	if err != nil {
		t.Fatalf("create test user %q: %v", username, err)
	}
	return id
}

func insertContactsTestRequest(
	t *testing.T,
	ctx context.Context,
	conn *pgx.Conn,
	senderID pgtype.UUID,
	recipientID pgtype.UUID,
	status string,
	createdAt time.Time,
) pgtype.UUID {
	t.Helper()

	var id pgtype.UUID
	err := conn.QueryRow(ctx, `
		INSERT INTO contact_requests (sender_id, recipient_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		RETURNING id`, senderID, recipientID, status, createdAt).Scan(&id)
	if err != nil {
		t.Fatalf("insert %s contact request: %v", status, err)
	}
	return id
}

func parseContactsTestUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()

	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		t.Fatalf("parse test UUID %q: %v", value, err)
	}
	return id
}
