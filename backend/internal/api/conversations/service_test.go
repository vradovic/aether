package conversations_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vradovic/aether/backend/internal/api/apitest"
	"github.com/vradovic/aether/backend/internal/api/conversations"
	"github.com/vradovic/aether/backend/internal/core"
)

func TestConversationsService(t *testing.T) {
	ctx := context.Background()
	pool, queries := apitest.StartDatabase(t, ctx)
	service := conversations.NewService(queries, slog.New(slog.DiscardHandler))

	ownerID := createContactsTestUser(t, ctx, pool, "conversation_owner")
	contactID := createContactsTestUser(t, ctx, pool, "conversation_contact")
	nonContactID := createContactsTestUser(t, ctx, pool, "conversation_non_contact")
	insertConversationTestContact(t, ctx, pool, ownerID, contactID)

	t.Run("create atomically adds the creator", func(t *testing.T) {
		conversation, err := service.CreateConversation(ctx, "  Team chat  ", ownerID.String())
		if err != nil {
			t.Fatalf("CreateConversation() error = %v", err)
		}
		if conversation.Name != "Team chat" || conversation.CreatedBy != ownerID.String() || conversation.LastMessageSequence != 0 {
			t.Fatalf("CreateConversation() = %+v", conversation)
		}

		var participantCount int
		if err := pool.QueryRow(ctx, `
			SELECT count(*)
			FROM conversation_participants
			WHERE conversation_id = $1 AND user_id = $2`, conversation.ID, ownerID).Scan(&participantCount); err != nil {
			t.Fatalf("count creator participation: %v", err)
		}
		if participantCount != 1 {
			t.Fatalf("creator participation count = %d, want 1", participantCount)
		}

		unnamed, err := service.CreateConversation(ctx, "", ownerID.String())
		if err != nil {
			t.Fatalf("create unnamed conversation: %v", err)
		}
		if unnamed.Name != "" {
			t.Fatalf("unnamed conversation name = %q, want empty", unnamed.Name)
		}
	})

	conversation, err := service.CreateConversation(ctx, "General", ownerID.String())
	if err != nil {
		t.Fatalf("seed conversation: %v", err)
	}

	t.Run("only the owner can update the name", func(t *testing.T) {
		updated, err := service.UpdateConversation(ctx, "  Project chat  ", ownerID.String(), conversation.ID)
		if err != nil {
			t.Fatalf("UpdateConversation() error = %v", err)
		}
		if updated.Name != "Project chat" {
			t.Fatalf("updated name = %q, want Project chat", updated.Name)
		}

		if _, err := service.UpdateConversation(ctx, "Nope", contactID.String(), conversation.ID); !errors.Is(err, conversations.ErrConversationNotFound) {
			t.Fatalf("non-owner update error = %v, want %v", err, conversations.ErrConversationNotFound)
		}
		if _, err := service.UpdateConversation(ctx, " ", ownerID.String(), conversation.ID); !errors.Is(err, conversations.ErrInvalidConversationName) {
			t.Fatalf("blank-name update error = %v, want %v", err, conversations.ErrInvalidConversationName)
		}
		if _, err := service.UpdateConversation(ctx, strings.Repeat("x", 51), ownerID.String(), conversation.ID); !errors.Is(err, conversations.ErrInvalidConversationName) {
			t.Fatalf("long-name update error = %v, want %v", err, conversations.ErrInvalidConversationName)
		}
	})

	t.Run("adds only owner contacts", func(t *testing.T) {
		participant, err := service.AddParticipant(ctx, ownerID.String(), conversation.ID, contactID.String())
		if err != nil {
			t.Fatalf("AddParticipant() error = %v", err)
		}
		if participant.ConversationID != conversation.ID || participant.UserID != contactID.String() {
			t.Fatalf("AddParticipant() = %+v", participant)
		}

		if _, err := service.AddParticipant(ctx, ownerID.String(), conversation.ID, contactID.String()); !errors.Is(err, conversations.ErrParticipantExists) {
			t.Fatalf("duplicate AddParticipant() error = %v, want %v", err, conversations.ErrParticipantExists)
		}
		if _, err := service.AddParticipant(ctx, ownerID.String(), conversation.ID, nonContactID.String()); !errors.Is(err, conversations.ErrParticipantNotContact) {
			t.Fatalf("non-contact AddParticipant() error = %v, want %v", err, conversations.ErrParticipantNotContact)
		}
		if _, err := service.AddParticipant(ctx, contactID.String(), conversation.ID, nonContactID.String()); !errors.Is(err, conversations.ErrConversationNotFound) {
			t.Fatalf("non-owner AddParticipant() error = %v, want %v", err, conversations.ErrConversationNotFound)
		}
	})

	t.Run("gets conversations only for participants", func(t *testing.T) {
		ownerConversations, err := service.GetConversations(ctx, ownerID.String())
		if err != nil {
			t.Fatalf("owner GetConversations() error = %v", err)
		}
		if !containsConversation(ownerConversations, conversation.ID, "Project chat") {
			t.Fatalf("owner conversations do not contain updated conversation: %+v", ownerConversations)
		}

		contactConversations, err := service.GetConversations(ctx, contactID.String())
		if err != nil {
			t.Fatalf("contact GetConversations() error = %v", err)
		}
		if len(contactConversations) != 1 || contactConversations[0].ID != conversation.ID {
			t.Fatalf("contact conversations = %+v, want only %s", contactConversations, conversation.ID)
		}

		nonContactConversations, err := service.GetConversations(ctx, nonContactID.String())
		if err != nil {
			t.Fatalf("non-contact GetConversations() error = %v", err)
		}
		if len(nonContactConversations) != 0 {
			t.Fatalf("non-contact conversations = %+v, want empty", nonContactConversations)
		}
	})

	t.Run("gets messages after the requested sequence for participants", func(t *testing.T) {
		seedConversationTestMessages(t, ctx, pool, conversation.ID, ownerID, contactID)

		messages, err := service.GetMessages(ctx, contactID.String(), conversation.ID, 1)
		if err != nil {
			t.Fatalf("GetMessages() error = %v", err)
		}
		if len(messages) != 2 || messages[0].MessageSequence != 3 || messages[1].MessageSequence != 2 {
			t.Fatalf("GetMessages() sequences = %+v, want [3, 2]", messages)
		}
		if messages[0].ConversationID != conversation.ID || messages[0].ClientMessageID == "" {
			t.Fatalf("message mapping is incomplete: %+v", messages[0])
		}

		if _, err := service.GetMessages(ctx, nonContactID.String(), conversation.ID, 0); !errors.Is(err, conversations.ErrConversationNotFound) {
			t.Fatalf("outsider GetMessages() error = %v, want %v", err, conversations.ErrConversationNotFound)
		}
		if _, err := service.GetMessages(ctx, ownerID.String(), conversation.ID, -1); !errors.Is(err, conversations.ErrInvalidAfterSequence) {
			t.Fatalf("negative sequence error = %v, want %v", err, conversations.ErrInvalidAfterSequence)
		}
	})

	t.Run("only the owner can remove non-owner participants", func(t *testing.T) {
		if err := service.RemoveParticipant(ctx, contactID.String(), conversation.ID, ownerID.String()); !errors.Is(err, conversations.ErrParticipantNotFound) {
			t.Fatalf("non-owner RemoveParticipant() error = %v, want %v", err, conversations.ErrParticipantNotFound)
		}
		if err := service.RemoveParticipant(ctx, ownerID.String(), conversation.ID, ownerID.String()); !errors.Is(err, conversations.ErrParticipantNotFound) {
			t.Fatalf("remove creator error = %v, want %v", err, conversations.ErrParticipantNotFound)
		}
		if err := service.RemoveParticipant(ctx, ownerID.String(), conversation.ID, contactID.String()); err != nil {
			t.Fatalf("RemoveParticipant() error = %v", err)
		}
		if _, err := service.GetMessages(ctx, contactID.String(), conversation.ID, 0); !errors.Is(err, conversations.ErrConversationNotFound) {
			t.Fatalf("removed participant GetMessages() error = %v, want %v", err, conversations.ErrConversationNotFound)
		}
	})

	t.Run("only the owner can delete and deletion cascades", func(t *testing.T) {
		if err := service.DeleteConversation(ctx, contactID.String(), conversation.ID); !errors.Is(err, conversations.ErrConversationNotFound) {
			t.Fatalf("non-owner DeleteConversation() error = %v, want %v", err, conversations.ErrConversationNotFound)
		}
		if err := service.DeleteConversation(ctx, ownerID.String(), conversation.ID); err != nil {
			t.Fatalf("DeleteConversation() error = %v", err)
		}

		var conversationsCount, participants, messages int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM conversations WHERE id = $1", conversation.ID).Scan(&conversationsCount); err != nil {
			t.Fatalf("count deleted conversation: %v", err)
		}
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM conversation_participants WHERE conversation_id = $1", conversation.ID).Scan(&participants); err != nil {
			t.Fatalf("count deleted participants: %v", err)
		}
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM messages WHERE conversation_id = $1", conversation.ID).Scan(&messages); err != nil {
			t.Fatalf("count deleted messages: %v", err)
		}
		if conversationsCount != 0 || participants != 0 || messages != 0 {
			t.Fatalf("delete counts = conversations:%d participants:%d messages:%d, want all zero", conversationsCount, participants, messages)
		}
		if err := service.DeleteConversation(ctx, ownerID.String(), conversation.ID); !errors.Is(err, conversations.ErrConversationNotFound) {
			t.Fatalf("repeated DeleteConversation() error = %v, want %v", err, conversations.ErrConversationNotFound)
		}
	})

	t.Run("rejects invalid IDs", func(t *testing.T) {
		if _, err := service.CreateConversation(ctx, "Name", "invalid"); !errors.Is(err, core.ErrInvalidID) {
			t.Fatalf("invalid creator error = %v, want %v", err, core.ErrInvalidID)
		}
		if _, err := service.GetMessages(ctx, ownerID.String(), "invalid", 0); !errors.Is(err, conversations.ErrInvalidConversationID) {
			t.Fatalf("invalid conversation error = %v, want %v", err, conversations.ErrInvalidConversationID)
		}
		if _, err := service.GetConversations(ctx, "invalid"); !errors.Is(err, core.ErrInvalidID) {
			t.Fatalf("invalid user error = %v, want %v", err, core.ErrInvalidID)
		}
	})
}

func createContactsTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, username string) pgtype.UUID {
	t.Helper()

	var id pgtype.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, username, password_hash, first_name, last_name)
		VALUES ($1, $2, 'unused-password-hash', 'Test', 'User')
		RETURNING id`, username+"@example.com", username).Scan(&id)
	if err != nil {
		t.Fatalf("create test user %q: %v", username, err)
	}
	return id
}

func insertConversationTestContact(t *testing.T, ctx context.Context, pool *pgxpool.Pool, user1ID, user2ID pgtype.UUID) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO contacts (user1_id, user2_id)
		VALUES (LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid))`, user1ID, user2ID); err != nil {
		t.Fatalf("insert conversation test contact: %v", err)
	}
}

func seedConversationTestMessages(t *testing.T, ctx context.Context, pool *pgxpool.Pool, conversationID string, ownerID, contactID pgtype.UUID) {
	t.Helper()
	for sequence, senderID := range []pgtype.UUID{ownerID, contactID, ownerID} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO messages (conversation_id, sender_id, client_message_id, message_sequence, body)
			VALUES ($1, $2, gen_random_uuid(), $3, $4)`, conversationID, senderID, sequence+1, "message body"); err != nil {
			t.Fatalf("insert message sequence %d: %v", sequence+1, err)
		}
	}
	if _, err := pool.Exec(ctx, "UPDATE conversations SET last_message_sequence = 3 WHERE id = $1", conversationID); err != nil {
		t.Fatalf("update last message sequence: %v", err)
	}
}

func containsConversation(convs []conversations.Conversation, id, name string) bool {
	for _, conversation := range convs {
		if conversation.ID == id && conversation.Name == name {
			return true
		}
	}
	return false
}
