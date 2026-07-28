package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/vradovic/aether/backend/internal/api"
	"github.com/vradovic/aether/backend/internal/core"
	"github.com/vradovic/aether/backend/internal/db"
)

const MinPasswordLengthBytes = 8
const MaxPasswordLengthBytes = 72
const MinNameLength = 2
const MaxNameLength = 50
const MinUsernameLength = 3
const MaxUsernameLength = 30

var ErrPasswordLength = fmt.Errorf("password should be between %d and %d characters long", MinPasswordLengthBytes, MaxPasswordLengthBytes)
var ErrNameLength = fmt.Errorf("name should be between %d and %d characters long", MinNameLength, MaxNameLength)
var ErrUsernameLength = fmt.Errorf("username should be between %d and %d characters long", MinUsernameLength, MaxUsernameLength)
var ErrEmailFormat = fmt.Errorf("invalid email format")

type RegisterInput struct {
	Email     string
	Username  string
	Password  string
	FirstName string
	LastName  string
}

type LoginInput struct {
	Email    string
	Password string
}

func (i LoginInput) Normalize() LoginInput {
	return LoginInput{
		Email:    strings.ToLower(strings.TrimSpace(i.Email)),
		Password: i.Password,
	}
}

type LoginOutput struct {
	AccessToken string
}

func (r RegisterInput) Normalize() RegisterInput {
	email := strings.ToLower(strings.TrimSpace(r.Email))
	username := strings.TrimSpace(r.Username)
	firstName := strings.TrimSpace(r.FirstName)
	lastName := strings.TrimSpace(r.LastName)

	return RegisterInput{
		Email:     email,
		Username:  username,
		Password:  r.Password,
		FirstName: firstName,
		LastName:  lastName,
	}
}

func (r RegisterInput) Validate() error {
	if utf8.RuneCountInString(r.FirstName) < MinNameLength ||
		utf8.RuneCountInString(r.FirstName) > MaxNameLength ||
		utf8.RuneCountInString(r.LastName) < MinNameLength ||
		utf8.RuneCountInString(r.LastName) > MaxNameLength {
		return ErrNameLength
	}

	addr, err := mail.ParseAddress(r.Email)
	if err != nil || addr.Address != r.Email {
		return ErrEmailFormat
	}

	if len(r.Password) < MinPasswordLengthBytes ||
		len(r.Password) > MaxPasswordLengthBytes {
		return ErrPasswordLength
	}

	if utf8.RuneCountInString(r.Username) < MinUsernameLength ||
		utf8.RuneCountInString(r.Username) > MaxUsernameLength {
		return ErrUsernameLength
	}

	return nil
}

type Querier interface {
	CreateUser(ctx context.Context, arg db.CreateUserParams) error
	GetUserCredentialsByEmail(ctx context.Context, email string) (db.GetUserCredentialsByEmailRow, error)
}

type Service struct {
	querier    Querier
	signingKey string
}

func NewService(queries Querier, signingKey string) *Service {
	return &Service{
		querier:    queries,
		signingKey: signingKey,
	}
}

func (s *Service) Login(ctx context.Context, input LoginInput) (LoginOutput, error) {
	input = input.Normalize()

	credentials, err := s.querier.GetUserCredentialsByEmail(ctx, input.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginOutput{}, api.ErrInvalidCredentials
	}
	if err != nil {
		return LoginOutput{}, fmt.Errorf("get user credentials: %w", err)
	}

	if err := api.VerifyPassword(input.Password, credentials.PasswordHash); err != nil {
		return LoginOutput{}, api.ErrInvalidCredentials
	}

	token, err := core.IssueToken(s.signingKey, credentials.UserID.String())
	if err != nil {
		return LoginOutput{}, fmt.Errorf("issue access token: %w", err)
	}

	return LoginOutput{
		AccessToken: token,
	}, nil
}

func (s *Service) Register(ctx context.Context, input RegisterInput) error {
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}

	passwordHash, err := api.HashPassword(input.Password)
	if err != nil {
		return err
	}

	return s.querier.CreateUser(ctx, db.CreateUserParams{
		Email:        input.Email,
		Username:     input.Username,
		PasswordHash: passwordHash,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
	})
}
