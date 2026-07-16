package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/vradovic/aether/services/api/internal/core"
	"github.com/vradovic/aether/services/api/internal/db"
)

const minPasswordLengthBytes = 8
const maxPasswordLengthBytes = 72
const minNameLength = 2
const maxNameLength = 50
const minUsernameLength = 3
const maxUsernameLength = 30

var errPasswordLength = fmt.Errorf("password should be between %d and %d characters long", minPasswordLengthBytes, maxPasswordLengthBytes)
var errNameLength = fmt.Errorf("name should be between %d and %d characters long", minNameLength, maxNameLength)
var errUsernameLength = fmt.Errorf("username should be between %d and %d characters long", minUsernameLength, maxUsernameLength)
var errEmailFormat = fmt.Errorf("invalid email format")

type registerInput struct {
	email     string
	username  string
	password  string
	firstName string
	lastName  string
}

type loginInput struct {
	email    string
	password string
}

func (i loginInput) normalize() loginInput {
	return loginInput{
		email:    strings.ToLower(strings.TrimSpace(i.email)),
		password: i.password,
	}
}

type loginOutput struct {
	accessToken      string
	expiresInSeconds int64
}

func (r registerInput) normalize() registerInput {
	email := strings.ToLower(strings.TrimSpace(r.email))
	username := strings.TrimSpace(r.username)
	firstName := strings.TrimSpace(r.firstName)
	lastName := strings.TrimSpace(r.lastName)

	return registerInput{
		email:     email,
		username:  username,
		password:  r.password,
		firstName: firstName,
		lastName:  lastName,
	}

}

func (r registerInput) validate() error {
	if utf8.RuneCountInString(r.firstName) < minNameLength ||
		utf8.RuneCountInString(r.firstName) > maxNameLength ||
		utf8.RuneCountInString(r.lastName) < minNameLength ||
		utf8.RuneCountInString(r.lastName) > maxNameLength {
		return errNameLength
	}

	addr, err := mail.ParseAddress(r.email)
	if err != nil || addr.Address != r.email { // Required to check Address because ParseAddress parses "Name <name@email.com>" as well
		return errEmailFormat
	}

	if len(r.password) < minPasswordLengthBytes ||
		len(r.password) > maxPasswordLengthBytes {
		return errPasswordLength
	}

	if utf8.RuneCountInString(r.username) < minUsernameLength ||
		utf8.RuneCountInString(r.username) > maxUsernameLength {
		return errUsernameLength
	}

	return nil
}

type authQuerier interface {
	CreateUser(ctx context.Context, arg db.CreateUserParams) error
	GetUserCredentialsByEmail(ctx context.Context, email string) (db.GetUserCredentialsByEmailRow, error)
}

type authService struct {
	querier     authQuerier
	tokenIssuer core.TokenIssuer
	logger      *slog.Logger
}

func NewAuthService(queries authQuerier, tokenIssuer core.TokenIssuer, logger *slog.Logger) *authService {
	return &authService{
		querier:     queries,
		tokenIssuer: tokenIssuer,
		logger:      logger,
	}
}

func (s *authService) login(ctx context.Context, input loginInput) (loginOutput, error) {
	input = input.normalize()

	credentials, err := s.querier.GetUserCredentialsByEmail(ctx, input.email)
	if errors.Is(err, pgx.ErrNoRows) {
		return loginOutput{}, errInvalidCredentials
	}
	if err != nil {
		return loginOutput{}, fmt.Errorf("get user credentials: %w", err)
	}

	if err := verifyPassword(input.password, credentials.PasswordHash); err != nil {
		return loginOutput{}, errInvalidCredentials
	}

	token, err := s.tokenIssuer.Issue(credentials.UserID.String())
	if err != nil {
		return loginOutput{}, fmt.Errorf("issue access token: %w", err)
	}

	return loginOutput{
		accessToken:      token.Value,
		expiresInSeconds: token.ExpiresInSeconds,
	}, nil
}

func (s *authService) register(ctx context.Context, input registerInput) error {
	input = input.normalize()
	if err := input.validate(); err != nil {
		return err
	}

	passwordHash, err := hashPassword(input.password)
	if err != nil {
		return err
	}

	return s.querier.CreateUser(ctx, db.CreateUserParams{
		Email:        input.email,
		Username:     input.username,
		PasswordHash: passwordHash,
		FirstName:    input.firstName,
		LastName:     input.lastName,
	})
}
