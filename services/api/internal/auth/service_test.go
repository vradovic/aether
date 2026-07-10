package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/db"
)

var testUserID = pgtype.UUID{
	Bytes: [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x00},
	Valid: true,
}

type fakeQuerier struct {
	calls           int
	params          db.CreateUserParams
	err             error
	credentialCalls int
	credentialEmail string
	credentials     db.GetUserCredentialsByEmailRow
	credentialErr   error
}

func (f *fakeQuerier) CreateUser(_ context.Context, params db.CreateUserParams) error {
	f.calls++
	f.params = params
	return f.err
}

func (f *fakeQuerier) GetUserCredentialsByEmail(_ context.Context, email string) (db.GetUserCredentialsByEmailRow, error) {
	f.credentialCalls++
	f.credentialEmail = email
	return f.credentials, f.credentialErr
}

type fakeTokenIssuer struct {
	calls  int
	userID string
	token  issuedToken
	err    error
}

func (f *fakeTokenIssuer) issue(userID string) (issuedToken, error) {
	f.calls++
	f.userID = userID
	return f.token, f.err
}

func TestLoginInputNormalize(t *testing.T) {
	input := loginInput{
		email:    "  User.Name@Example.COM\t",
		password: " password with spaces ",
	}

	want := loginInput{
		email:    "user.name@example.com",
		password: " password with spaces ",
	}

	if got := input.normalize(); got != want {
		t.Fatalf("normalize() = %#v, want %#v", got, want)
	}
}

func TestServiceLogin(t *testing.T) {
	password := " password123 "
	passwordHash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	t.Run("normalizes email and returns an access token", func(t *testing.T) {
		querier := &fakeQuerier{
			credentials: db.GetUserCredentialsByEmailRow{
				UserID:       testUserID,
				PasswordHash: passwordHash,
			},
		}
		tokens := &fakeTokenIssuer{
			token: issuedToken{
				value:            "signed-token",
				expiresInSeconds: 900,
			},
		}
		svc := &service{
			querier:     querier,
			tokenIssuer: tokens,
		}

		output, err := svc.login(context.Background(), loginInput{
			email:    "  User@Example.COM ",
			password: password,
		})
		if err != nil {
			t.Fatalf("login() error = %v", err)
		}
		if querier.credentialEmail != "user@example.com" {
			t.Errorf("GetUserCredentialsByEmail() email = %q, want %q", querier.credentialEmail, "user@example.com")
		}
		if tokens.calls != 1 {
			t.Fatalf("issue() calls = %d, want 1", tokens.calls)
		}
		if tokens.userID != querier.credentials.UserID.String() {
			t.Errorf("issue() user ID = %q, want %q", tokens.userID, querier.credentials.UserID.String())
		}
		if output.accessToken != tokens.token.value {
			t.Errorf("login() access token = %q, want %q", output.accessToken, tokens.token.value)
		}
		if output.expiresInSeconds != tokens.token.expiresInSeconds {
			t.Errorf("login() expires in = %d, want %d", output.expiresInSeconds, tokens.token.expiresInSeconds)
		}
	})

	t.Run("incorrect password returns invalid credentials", func(t *testing.T) {
		querier := &fakeQuerier{
			credentials: db.GetUserCredentialsByEmailRow{
				UserID:       testUserID,
				PasswordHash: passwordHash,
			},
		}
		tokens := &fakeTokenIssuer{}
		svc := &service{
			querier:     querier,
			tokenIssuer: tokens,
		}

		_, err := svc.login(context.Background(), loginInput{
			email:    "user@example.com",
			password: "wrong password",
		})
		if !errors.Is(err, errInvalidCredentials) {
			t.Fatalf("login() error = %v, want %v", err, errInvalidCredentials)
		}
		if tokens.calls != 0 {
			t.Fatalf("issue() calls = %d, want 0", tokens.calls)
		}
	})

	t.Run("unknown email returns invalid credentials", func(t *testing.T) {
		querier := &fakeQuerier{credentialErr: pgx.ErrNoRows}
		tokens := &fakeTokenIssuer{}
		svc := &service{
			querier:     querier,
			tokenIssuer: tokens,
		}

		_, err := svc.login(context.Background(), loginInput{
			email:    "missing@example.com",
			password: password,
		})
		if !errors.Is(err, errInvalidCredentials) {
			t.Fatalf("login() error = %v, want %v", err, errInvalidCredentials)
		}
		if tokens.calls != 0 {
			t.Fatalf("issue() calls = %d, want 0", tokens.calls)
		}
	})

	t.Run("returns database error", func(t *testing.T) {
		dbErr := errors.New("database unavailable")
		querier := &fakeQuerier{credentialErr: dbErr}
		tokens := &fakeTokenIssuer{}
		svc := &service{
			querier:     querier,
			tokenIssuer: tokens,
		}

		_, err := svc.login(context.Background(), loginInput{
			email:    "user@example.com",
			password: password,
		})
		if !errors.Is(err, dbErr) {
			t.Fatalf("login() error = %v, want %v", err, dbErr)
		}
		if tokens.calls != 0 {
			t.Fatalf("issue() calls = %d, want 0", tokens.calls)
		}
	})

	t.Run("returns token issuer error", func(t *testing.T) {
		issuerErr := errors.New("signing unavailable")
		querier := &fakeQuerier{
			credentials: db.GetUserCredentialsByEmailRow{
				UserID:       testUserID,
				PasswordHash: passwordHash,
			},
		}
		tokens := &fakeTokenIssuer{err: issuerErr}
		svc := &service{
			querier:     querier,
			tokenIssuer: tokens,
		}

		_, err := svc.login(context.Background(), loginInput{
			email:    "user@example.com",
			password: password,
		})
		if !errors.Is(err, issuerErr) {
			t.Fatalf("login() error = %v, want %v", err, issuerErr)
		}
	})
}

func TestRegisterInputNormalize(t *testing.T) {
	tests := []struct {
		name  string
		input registerInput
		want  registerInput
	}{
		{
			name: "normalizes email and trims username and names",
			input: registerInput{
				email:     "  User.Name@Example.COM\t",
				username:  "  petar_92 ",
				password:  " password with spaces ",
				firstName: "  Petar ",
				lastName:  "\tPetrović\n",
			},
			want: registerInput{
				email:     "user.name@example.com",
				username:  "petar_92",
				password:  " password with spaces ",
				firstName: "Petar",
				lastName:  "Petrović",
			},
		},
		{
			name: "leaves normalized input unchanged",
			input: registerInput{
				email:     "user@example.com",
				username:  "ana",
				password:  "password123",
				firstName: "Ana",
				lastName:  "Ivić",
			},
			want: registerInput{
				email:     "user@example.com",
				username:  "ana",
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
		username:  "petar",
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
			name: "username is too short",
			input: registerInput{
				email:     valid.email,
				username:  "ab",
				password:  valid.password,
				firstName: valid.firstName,
				lastName:  valid.lastName,
			},
			want: errUsernameLength,
		},
		{
			name: "username is too long",
			input: registerInput{
				email:     valid.email,
				username:  strings.Repeat("a", maxUsernameLength+1),
				password:  valid.password,
				firstName: valid.firstName,
				lastName:  valid.lastName,
			},
			want: errUsernameLength,
		},
		{
			name: "unicode names are counted as characters",
			input: registerInput{
				email:     valid.email,
				username:  valid.username,
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
				username:  valid.username,
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
			username:  " petar_92 ",
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
		if querier.params.Username != "petar_92" {
			t.Errorf("CreateUser() username = %q, want %q", querier.params.Username, "petar_92")
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
			username:  "petar",
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
			username:  "petar",
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
