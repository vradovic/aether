package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/vradovic/aether/services/api/internal/db"
)

const minPasswordLengthBytes = 8
const maxPasswordLengthBytes = 72
const minNameLength = 2
const maxNameLength = 50

var errPasswordLength = fmt.Errorf("password should be between %d and %d characters long", minPasswordLengthBytes, maxPasswordLengthBytes)
var errNameLength = fmt.Errorf("name should be between %d and %d characters long", minNameLength, maxNameLength)
var errEmailFormat = fmt.Errorf("invalid email format")

type registerInput struct {
	email     string
	password  string
	firstName string
	lastName  string
}

func (r registerInput) normalize() registerInput {
	email := strings.ToLower(strings.TrimSpace(r.email))
	firstName := strings.TrimSpace(r.firstName)
	lastName := strings.TrimSpace(r.lastName)

	return registerInput{
		email:     email,
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

	return nil
}

type querier interface {
	CreateUser(ctx context.Context, arg db.CreateUserParams) error
}

type service struct {
	querier querier
	logger  *slog.Logger
}

func NewService(queries querier, logger *slog.Logger) *service {
	return &service{
		querier: queries,
		logger:  logger,
	}
}

func (s *service) register(ctx context.Context, input registerInput) error {
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
		PasswordHash: passwordHash,
		FirstName:    input.firstName,
		LastName:     input.lastName,
	})
}
