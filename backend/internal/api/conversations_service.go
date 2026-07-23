package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/core"
	"github.com/vradovic/aether/services/api/internal/db"
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
	ErrInvalidAfterSequence    = errors.New("after_sequence must be a non-negative integer")
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
	MessageSequence int64     `json:"messageSequence"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type ConversationParticipant struct {
	ConversationID string    `json:"conversationId"`
	UserID         string    `json:"userId"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type conversationsQuerier interface {
	GetConversationsForUser(context.Context, pgtype.UUID) ([]db.Conversation, error)
	CreateConversationWithCreator(context.Context, db.CreateConversationWithCreatorParams) (db.CreateConversationWithCreatorRow, error)
	UpdateConversationName(context.Context, db.UpdateConversationNameParams) (db.Conversation, error)
	DeleteConversation(context.Context, db.DeleteConversationParams) (pgtype.UUID, error)
	SyncMessages(context.Context, db.SyncMessagesParams) ([]db.Message, error)
	IsConversationParticipant(context.Context, db.IsConversationParticipantParams) (bool, error)
	IsConversationOwner(context.Context, db.IsConversationOwnerParams) (bool, error)
	AreContacts(context.Context, db.AreContactsParams) (bool, error)
	InsertConversationParticipant(context.Context, db.InsertConversationParticipantParams) (db.ConversationParticipant, error)
	DeleteConversationParticipant(context.Context, db.DeleteConversationParticipantParams) (db.ConversationParticipant, error)
}

type conversationsService struct {
	querier conversationsQuerier
	logger  *slog.Logger
}

func NewConversationsService(querier conversationsQuerier, logger *slog.Logger) *conversationsService {
	return &conversationsService{querier: querier, logger: logger}
}

func (s *conversationsService) GetConversations(ctx context.Context, userID string) ([]Conversation, error) {
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

func (s *conversationsService) CreateConversation(ctx context.Context, name, userID string) (Conversation, error) {
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

func (s *conversationsService) UpdateConversation(ctx context.Context, name, userID, conversationID string) (Conversation, error) {
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

func (s *conversationsService) GetMessages(ctx context.Context, userID, conversationID string, afterSequence int64) ([]Message, error) {
	if afterSequence < 0 {
		return nil, ErrInvalidAfterSequence
	}
	participantID, err := core.ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	conversationUUID, err := core.ParseUUID(conversationID)
	if err != nil {
		return nil, ErrInvalidConversationID
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

	databaseMessages, err := s.querier.SyncMessages(ctx, db.SyncMessagesParams{
		UserID:         participantID,
		ConversationID: conversationUUID,
		AfterSequence:  afterSequence,
		PageSize:       messagesPageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("get messages for conversation %s: %w", conversationID, err)
	}

	messages := make([]Message, 0, len(databaseMessages))
	for _, message := range databaseMessages {
		messages = append(messages, Message{
			ID:              message.ID.String(),
			ConversationID:  message.ConversationID.String(),
			SenderID:        message.SenderID.String(),
			ClientMessageID: message.ClientMessageID.String(),
			MessageSequence: message.MessageSequence,
			Body:            message.Body,
			CreatedAt:       message.CreatedAt.Time,
			UpdatedAt:       message.UpdatedAt.Time,
		})
	}
	return messages, nil
}

func (s *conversationsService) AddParticipant(ctx context.Context, userID, conversationID, participantID string) (ConversationParticipant, error) {
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

func (s *conversationsService) DeleteConversation(ctx context.Context, userID, conversationID string) error {
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

func (s *conversationsService) RemoveParticipant(ctx context.Context, userID, conversationID, participantID string) error {
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
