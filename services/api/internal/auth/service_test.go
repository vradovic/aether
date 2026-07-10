package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vradovic/aether/services/api/internal/db"
)

type fakeQuerier struct {
	calls  int
	params db.CreateUserParams
	err    error
}

func (f *fakeQuerier) CreateUser(_ context.Context, params db.CreateUserParams) error {
	f.calls++
	f.params = params
	return f.err
}

func TestRegisterInputNormalize(t *testing.T) {
	tests := []struct {
		name  string
		input registerInput
		want  registerInput
	}{
		{
			name: "trims and lowercases email and trims names",
			input: registerInput{
				email:     "  User.Name@Example.COM\t",
				password:  " password with spaces ",
				firstName: "  Petar ",
				lastName:  "\tPetrović\n",
			},
			want: registerInput{
				email:     "user.name@example.com",
				password:  " password with spaces ",
				firstName: "Petar",
				lastName:  "Petrović",
			},
		},
		{
			name: "leaves normalized input unchanged",
			input: registerInput{
				email:     "user@example.com",
				password:  "password123",
				firstName: "Ana",
				lastName:  "Ivić",
			},
			want: registerInput{
				email:     "user@example.com",
				password:  "password123",
				firstName: "Ana",
				lastName:  "Ivić",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.input
			got := tt.input.normalize()

			if got != tt.want {
				t.Fatalf("normalize() = %#v, want %#v", got, tt.want)
			}
			if tt.input != original {
				t.Fatalf("normalize() mutated its receiver: got %#v, want %#v", tt.input, original)
			}
		})
	}
}

func TestRegisterInputValidate(t *testing.T) {
	valid := registerInput{
		email:     "user@example.com",
		password:  "password123",
		firstName: "Petar",
		lastName:  "Petrović",
	}

	tests := []struct {
		name  string
		input registerInput
		want  error
	}{
		{
			name:  "valid input",
			input: valid,
		},
		{
			name: "unicode names are counted as characters",
			input: registerInput{
				email:     valid.email,
				password:  valid.password,
				firstName: "Žž",
				lastName:  strings.Repeat("ć", maxNameLength),
			},
		},
		{
			name: "first name is too short",
			input: registerInput{
				email:     valid.email,
				password:  valid.password,
				firstName: "A",
				lastName:  valid.lastName,
			},
			want: errNameLength,
		},
		{
			name: "last name is too long",
			input: registerInput{
				email:     valid.email,
				password:  valid.password,
				firstName: valid.firstName,
				lastName:  strings.Repeat("a", maxNameLength+1),
			},
			want: errNameLength,
		},
		{
			name: "email is malformed",
			input: registerInput{
				email:     "not-an-email",
				password:  valid.password,
				firstName: valid.firstName,
				lastName:  valid.lastName,
			},
			want: errEmailFormat,
		},
		{
			name: "email display name is rejected",
			input: registerInput{
				email:     "Petar <user@example.com>",
				password:  valid.password,
				firstName: valid.firstName,
				lastName:  valid.lastName,
			},
			want: errEmailFormat,
		},
		{
			name: "password is too short",
			input: registerInput{
				email:     valid.email,
				password:  strings.Repeat("a", minPasswordLengthBytes-1),
				firstName: valid.firstName,
				lastName:  valid.lastName,
			},
			want: errPasswordLength,
		},
		{
			name: "password at maximum byte length is valid",
			input: registerInput{
				email:     valid.email,
				password:  strings.Repeat("a", maxPasswordLengthBytes),
				firstName: valid.firstName,
				lastName:  valid.lastName,
			},
		},
		{
			name: "password exceeding bcrypt byte limit is rejected",
			input: registerInput{
				email:     valid.email,
				password:  strings.Repeat("é", maxPasswordLengthBytes/2+1),
				firstName: valid.firstName,
				lastName:  valid.lastName,
			},
			want: errPasswordLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.validate()
			if !errors.Is(err, tt.want) {
				t.Fatalf("validate() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestServiceRegister(t *testing.T) {
	t.Run("normalizes and creates user with hashed password", func(t *testing.T) {
		querier := &fakeQuerier{}
		svc := &service{querier: querier}
		input := registerInput{
			email:     "  User@Example.COM ",
			password:  " password123 ",
			firstName: "  Petar ",
			lastName:  " Petrović\t",
		}

		err := svc.register(context.Background(), input)
		if err != nil {
			t.Fatalf("register() error = %v", err)
		}
		if querier.calls != 1 {
			t.Fatalf("CreateUser() calls = %d, want 1", querier.calls)
		}
		if querier.params.Email != "user@example.com" {
			t.Errorf("CreateUser() email = %q, want %q", querier.params.Email, "user@example.com")
		}
		if querier.params.FirstName != "Petar" {
			t.Errorf("CreateUser() first name = %q, want %q", querier.params.FirstName, "Petar")
		}
		if querier.params.LastName != "Petrović" {
			t.Errorf("CreateUser() last name = %q, want %q", querier.params.LastName, "Petrović")
		}
		if querier.params.PasswordHash == input.password {
			t.Error("CreateUser() received the plaintext password")
		}
		if err := verifyPassword(input.password, querier.params.PasswordHash); err != nil {
			t.Errorf("stored password hash does not match the input password: %v", err)
		}
	})

	t.Run("validation error prevents user creation", func(t *testing.T) {
		querier := &fakeQuerier{}
		svc := &service{querier: querier}
		input := registerInput{
			email:     "user@example.com",
			password:  "password123",
			firstName: "   ",
			lastName:  "Petrović",
		}

		err := svc.register(context.Background(), input)
		if !errors.Is(err, errNameLength) {
			t.Fatalf("register() error = %v, want %v", err, errNameLength)
		}
		if querier.calls != 0 {
			t.Fatalf("CreateUser() calls = %d, want 0", querier.calls)
		}
	})

	t.Run("returns database error", func(t *testing.T) {
		dbErr := errors.New("database unavailable")
		querier := &fakeQuerier{err: dbErr}
		svc := &service{querier: querier}
		input := registerInput{
			email:     "user@example.com",
			password:  "password123",
			firstName: "Petar",
			lastName:  "Petrović",
		}

		err := svc.register(context.Background(), input)
		if !errors.Is(err, dbErr) {
			t.Fatalf("register() error = %v, want %v", err, dbErr)
		}
		if querier.calls != 1 {
			t.Fatalf("CreateUser() calls = %d, want 1", querier.calls)
		}
	})
}
