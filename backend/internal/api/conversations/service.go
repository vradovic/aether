package conversations

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gocql/gocql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/backend/internal/core"
	"github.com/vradovic/aether/backend/internal/db"
)

const (
	maxConversationNameLength = 50
	messagesPageSize          = 100
)

var (
	ErrConversationNotFound    = errors.New("conversation not found")
	ErrInvalidConversationID   = errors.New("invalid conversation ID")
	ErrInvalidParticipantID    = errors.New("invalid participant ID")
	ErrInvalidConversationName = errors.New("conversation name must contain between 1 and 50 characters")
	ErrInvalidAfterID          = errors.New("after_id must be a time UUID")
	ErrParticipantNotContact   = errors.New("conversation participants must be contacts")
	ErrParticipantExists       = errors.New("conversation participant already exists")
	ErrParticipantNotFound     = errors.New("conversation participant not found")
)

type Conversation struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	CreatedBy           string `json:"createdBy"`
	LastMessageSequence int64  `json:"lastMessageSequence"`
}

type Message struct {
	ID              string    `json:"id"`
	ConversationID  string    `json:"conversationId"`
	SenderID        string    `json:"senderId"`
	ClientMessageID string    `json:"clientMessageId"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"createdAt"`
}

// StoredMessage is a message as the worker service persisted it.
type StoredMessage struct {
	ID              gocql.UUID
	ConversationID  gocql.UUID
	SenderID        gocql.UUID
	ClientMessageID gocql.UUID
	Body            string
}

type ConversationParticipant struct {
	ConversationID string    `json:"conversationId"`
	UserID         string    `json:"userId"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Querier interface {
	GetConversationsForUser(context.Context, pgtype.UUID) ([]db.Conversation, error)
	CreateConversationWithCreator(context.Context, db.CreateConversationWithCreatorParams) (db.CreateConversationWithCreatorRow, error)
	UpdateConversationName(context.Context, db.UpdateConversationNameParams) (db.Conversation, error)
	DeleteConversation(context.Context, db.DeleteConversationParams) (pgtype.UUID, error)
	IsConversationParticipant(context.Context, db.IsConversationParticipantParams) (bool, error)
	IsConversationOwner(context.Context, db.IsConversationOwnerParams) (bool, error)
	AreContacts(context.Context, db.AreContactsParams) (bool, error)
	InsertConversationParticipant(context.Context, db.InsertConversationParticipantParams) (db.ConversationParticipant, error)
	DeleteConversationParticipant(context.Context, db.DeleteConversationParticipantParams) (db.ConversationParticipant, error)
}

// MessageStore reads persisted messages, oldest first. Conversations own their
// messages in ScyllaDB, so message reads do not go through the Querier.
type MessageStore interface {
	// ReadMessages returns at most pageSize messages of a conversation that
	// sort after afterID. A zero afterID starts from the oldest message.
	ReadMessages(ctx context.Context, conversationID, afterID gocql.UUID, pageSize int) ([]StoredMessage, error)
}

type Service struct {
	querier  Querier
	messages MessageStore
	logger   *slog.Logger
}

func NewService(querier Querier, messages MessageStore, logger *slog.Logger) *Service {
	return &Service{querier: querier, messages: messages, logger: logger}
}

func (s *Service) GetConversations(ctx context.Context, userID string) ([]Conversation, error) {
	parsedUserID, err := core.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	databaseConversations, err := s.querier.GetConversationsForUser(ctx, parsedUserID)
	if err != nil {
		return nil, fmt.Errorf("get conversations for user %s: %w", userID, err)
	}

	conversations := make([]Conversation, 0, len(databaseConversations))
	for _, databaseConversation := range databaseConversations {
		name := ""
		if databaseConversation.Name.Valid {
			name = databaseConversation.Name.String
		}
		conversations = append(conversations, Conversation{
			ID:                  databaseConversation.ID.String(),
			Name:                name,
			CreatedBy:           databaseConversation.CreatedBy.String(),
			LastMessageSequence: databaseConversation.LastMessageSequence,
		})
	}
	return conversations, nil
}

func (s *Service) CreateConversation(ctx context.Context, name, userID string) (Conversation, error) {
	creatorID, err := core.ParseUUID(userID)
	if err != nil {
		return Conversation{}, err
	}
	databaseName, err := normalizeConversationName(name, false)
	if err != nil {
		return Conversation{}, err
	}

	created, err := s.querier.CreateConversationWithCreator(ctx, db.CreateConversationWithCreatorParams{
		Name:      databaseName,
		CreatedBy: creatorID,
	})
	if err != nil {
		return Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	createdName := ""
	if created.Name.Valid {
		createdName = created.Name.String
	}

	return Conversation{
		ID:                  created.ID.String(),
		Name:                createdName,
		CreatedBy:           created.CreatedBy.String(),
		LastMessageSequence: created.LastMessageSequence,
	}, nil
}

func (s *Service) UpdateConversation(ctx context.Context, name, userID, conversationID string) (Conversation, error) {
	creatorID, err := core.ParseUUID(userID)
	if err != nil {
		return Conversation{}, err
	}
	conversationUUID, err := core.ParseUUID(conversationID)
	if err != nil {
		return Conversation{}, ErrInvalidConversationID
	}
	databaseName, err := normalizeConversationName(name, true)
	if err != nil {
		return Conversation{}, err
	}

	updated, err := s.querier.UpdateConversationName(ctx, db.UpdateConversationNameParams{
		Name:      databaseName,
		ID:        conversationUUID,
		CreatedBy: creatorID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, ErrConversationNotFound
	}
	if err != nil {
		return Conversation{}, fmt.Errorf("update conversation %s: %w", conversationID, err)
	}
	updatedName := ""
	if updated.Name.Valid {
		updatedName = updated.Name.String
	}
	return Conversation{
		ID:                  updated.ID.String(),
		Name:                updatedName,
		CreatedBy:           updated.CreatedBy.String(),
		LastMessageSequence: updated.LastMessageSequence,
	}, nil
}

func (s *Service) GetMessages(ctx context.Context, userID, conversationID, afterID string) ([]Message, error) {
	participantID, err := core.ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	conversationUUID, err := core.ParseUUID(conversationID)
	if err != nil {
		return nil, ErrInvalidConversationID
	}
	after, err := parseMessageCursor(afterID)
	if err != nil {
		return nil, err
	}

	isParticipant, err := s.querier.IsConversationParticipant(ctx, db.IsConversationParticipantParams{
		ConversationID: conversationUUID,
		UserID:         participantID,
	})
	if err != nil {
		return nil, fmt.Errorf("authorize messages for conversation %s: %w", conversationID, err)
	}
	if !isParticipant {
		return nil, ErrConversationNotFound
	}

	storedMessages, err := s.messages.ReadMessages(ctx, gocql.UUID(conversationUUID.Bytes), after, messagesPageSize)
	if err != nil {
		return nil, fmt.Errorf("get messages for conversation %s: %w", conversationID, err)
	}

	messages := make([]Message, 0, len(storedMessages))
	for _, message := range storedMessages {
		messages = append(messages, Message{
			ID:              message.ID.String(),
			ConversationID:  message.ConversationID.String(),
			SenderID:        message.SenderID.String(),
			ClientMessageID: message.ClientMessageID.String(),
			Body:            message.Body,
			CreatedAt:       message.ID.Time(),
		})
	}
	return messages, nil
}

// parseMessageCursor turns an after_id value into a clustering-key cursor. An
// empty value pages from the oldest message; anything that is not a time UUID
// cannot address a row in the message store.
func parseMessageCursor(afterID string) (gocql.UUID, error) {
	if afterID == "" {
		return gocql.UUID{}, nil
	}
	cursor, err := gocql.ParseUUID(afterID)
	if err != nil || cursor.Version() != 1 {
		return gocql.UUID{}, ErrInvalidAfterID
	}
	return cursor, nil
}

func (s *Service) AddParticipant(ctx context.Context, userID, conversationID, participantID string) (ConversationParticipant, error) {
	creatorID, err := core.ParseUUID(userID)
	if err != nil {
		return ConversationParticipant{}, err
	}
	conversationUUID, err := core.ParseUUID(conversationID)
	if err != nil {
		return ConversationParticipant{}, ErrInvalidConversationID
	}
	participantUUID, err := core.ParseUUID(participantID)
	if err != nil {
		return ConversationParticipant{}, ErrInvalidParticipantID
	}

	isOwner, err := s.querier.IsConversationOwner(ctx, db.IsConversationOwnerParams{
		ID:        conversationUUID,
		CreatedBy: creatorID,
	})
	if err != nil {
		return ConversationParticipant{}, fmt.Errorf("authorize participant addition for conversation %s: %w", conversationID, err)
	}
	if !isOwner {
		return ConversationParticipant{}, ErrConversationNotFound
	}
	if participantUUID == creatorID {
		return ConversationParticipant{}, ErrParticipantExists
	}

	areContacts, err := s.querier.AreContacts(ctx, db.AreContactsParams{
		UserID:    creatorID,
		ContactID: participantUUID,
	})
	if err != nil {
		return ConversationParticipant{}, fmt.Errorf("check contact before adding conversation participant: %w", err)
	}
	if !areContacts {
		return ConversationParticipant{}, ErrParticipantNotContact
	}

	participant, err := s.querier.InsertConversationParticipant(ctx, db.InsertConversationParticipantParams{
		ConversationID: conversationUUID,
		UserID:         participantUUID,
		CreatedBy:      creatorID,
	})
	if db.IsUniqueViolation(err) {
		return ConversationParticipant{}, ErrParticipantExists
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ConversationParticipant{}, ErrConversationNotFound
	}
	if err != nil {
		return ConversationParticipant{}, fmt.Errorf("add participant to conversation %s: %w", conversationID, err)
	}
	return ConversationParticipant{
		ConversationID: participant.ConversationID.String(),
		UserID:         participant.UserID.String(),
		CreatedAt:      participant.CreatedAt.Time,
		UpdatedAt:      participant.UpdatedAt.Time,
	}, nil
}

func (s *Service) DeleteConversation(ctx context.Context, userID, conversationID string) error {
	creatorID, err := core.ParseUUID(userID)
	if err != nil {
		return err
	}
	conversationUUID, err := core.ParseUUID(conversationID)
	if err != nil {
		return ErrInvalidConversationID
	}
	_, err = s.querier.DeleteConversation(ctx, db.DeleteConversationParams{
		ID:        conversationUUID,
		CreatedBy: creatorID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConversationNotFound
	}
	if err != nil {
		return fmt.Errorf("delete conversation %s: %w", conversationID, err)
	}
	return nil
}

func (s *Service) RemoveParticipant(ctx context.Context, userID, conversationID, participantID string) error {
	creatorID, err := core.ParseUUID(userID)
	if err != nil {
		return err
	}
	conversationUUID, err := core.ParseUUID(conversationID)
	if err != nil {
		return ErrInvalidConversationID
	}
	participantUUID, err := core.ParseUUID(participantID)
	if err != nil {
		return ErrInvalidParticipantID
	}

	_, err = s.querier.DeleteConversationParticipant(ctx, db.DeleteConversationParticipantParams{
		ConversationID: conversationUUID,
		UserID:         participantUUID,
		CreatedBy:      creatorID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrParticipantNotFound
	}
	if err != nil {
		return fmt.Errorf("remove participant from conversation %s: %w", conversationID, err)
	}
	return nil
}

func normalizeConversationName(name string, required bool) (pgtype.Text, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		if required {
			return pgtype.Text{}, ErrInvalidConversationName
		}
		return pgtype.Text{}, nil
	}
	if utf8.RuneCountInString(name) > maxConversationNameLength {
		return pgtype.Text{}, ErrInvalidConversationName
	}
	return pgtype.Text{String: name, Valid: true}, nil
}
