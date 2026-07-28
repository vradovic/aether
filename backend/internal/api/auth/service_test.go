package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/vradovic/aether/backend/internal/api"
	"github.com/vradovic/aether/backend/internal/api/apitest"
	"github.com/vradovic/aether/backend/internal/api/auth"
	"github.com/vradovic/aether/backend/internal/core"
)

func TestAuthService(t *testing.T) {
	ctx := context.Background()
	pool, queries := apitest.StartDatabase(t, ctx)
	service := auth.NewService(queries, apitest.TestSigningKey)

	t.Run("register", func(t *testing.T) {
		t.Run("stores normalized user and hashed password", func(t *testing.T) {
			input := auth.RegisterInput{
				Email:     "  Alice.Example@EXAMPLE.COM ",
				Username:  "  alice_example  ",
				Password:  "correct-horse-battery-staple",
				FirstName: "  Alice  ",
				LastName:  "  Example  ",
			}

			if err := service.Register(ctx, input); err != nil {
				t.Fatalf("Register() error = %v", err)
			}

			var email, username, passwordHash, firstName, lastName string
			err := pool.QueryRow(ctx, `
				SELECT email, username, password_hash, first_name, last_name
				FROM users
				WHERE email = $1`, "alice.example@example.com").Scan(
				&email, &username, &passwordHash, &firstName, &lastName,
			)
			if err != nil {
				t.Fatalf("query registered user: %v", err)
			}

			if email != "alice.example@example.com" || username != "alice_example" ||
				firstName != "Alice" || lastName != "Example" {
				t.Fatalf("user was not normalized: email=%q username=%q firstName=%q lastName=%q",
					email, username, firstName, lastName)
			}
			if passwordHash == input.Password {
				t.Fatal("password was stored as plaintext")
			}
			if err := api.VerifyPassword(input.Password, passwordHash); err != nil {
				t.Fatalf("stored password hash does not match password: %v", err)
			}
		})

		t.Run("rejects invalid input without writing", func(t *testing.T) {
			tests := []struct {
				name   string
				mutate func(*auth.RegisterInput)
				want   error
			}{
				{name: "invalid email", mutate: func(i *auth.RegisterInput) { i.Email = "not-an-email" }, want: auth.ErrEmailFormat},
				{name: "short password", mutate: func(i *auth.RegisterInput) { i.Password = "short" }, want: auth.ErrPasswordLength},
				{name: "long password", mutate: func(i *auth.RegisterInput) { i.Password = strings.Repeat("a", auth.MaxPasswordLengthBytes+1) }, want: auth.ErrPasswordLength},
				{name: "short username", mutate: func(i *auth.RegisterInput) { i.Username = "ab" }, want: auth.ErrUsernameLength},
				{name: "long username", mutate: func(i *auth.RegisterInput) { i.Username = strings.Repeat("a", auth.MaxUsernameLength+1) }, want: auth.ErrUsernameLength},
				{name: "short first name", mutate: func(i *auth.RegisterInput) { i.FirstName = "A" }, want: auth.ErrNameLength},
				{name: "long last name", mutate: func(i *auth.RegisterInput) { i.LastName = strings.Repeat("a", auth.MaxNameLength+1) }, want: auth.ErrNameLength},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					slug := strings.ReplaceAll(tt.name, " ", "-")
					input := validRegisterInput("validation-"+slug+"@example.com", "validation-"+slug)
					tt.mutate(&input)

					err := service.Register(ctx, input)
					if !errors.Is(err, tt.want) {
						t.Fatalf("Register() error = %v, want %v", err, tt.want)
					}

					var count int
					if err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE email = $1", input.Normalize().Email).Scan(&count); err != nil {
						t.Fatalf("count users: %v", err)
					}
					if count != 0 {
						t.Fatalf("invalid registration wrote %d users", count)
					}
				})
			}
		})

		t.Run("returns PostgreSQL unique constraint errors", func(t *testing.T) {
			base := validRegisterInput("unique@example.com", "unique_user")
			if err := service.Register(ctx, base); err != nil {
				t.Fatalf("seed registration: %v", err)
			}

			tests := []struct {
				name       string
				input      auth.RegisterInput
				constraint string
			}{
				{
					name:       "duplicate email",
					input:      validRegisterInput(" UNIQUE@EXAMPLE.COM ", "different_user"),
					constraint: "users_email_key",
				},
				{
					name:       "duplicate username",
					input:      validRegisterInput("different@example.com", " unique_user "),
					constraint: "users_username_key",
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					err := service.Register(ctx, tt.input)
					var pgErr *pgconn.PgError
					if !errors.As(err, &pgErr) {
						t.Fatalf("Register() error = %v, want PostgreSQL error", err)
					}
					if pgErr.Code != "23505" || pgErr.ConstraintName != tt.constraint {
						t.Fatalf("PostgreSQL error code=%q constraint=%q, want code=23505 constraint=%q",
							pgErr.Code, pgErr.ConstraintName, tt.constraint)
					}
				})
			}
		})
	})

	t.Run("login", func(t *testing.T) {
		input := validRegisterInput("login@example.com", "login_user")
		if err := service.Register(ctx, input); err != nil {
			t.Fatalf("seed registration: %v", err)
		}

		t.Run("returns access token for normalized email", func(t *testing.T) {
			output, err := service.Login(ctx, auth.LoginInput{
				Email:    "  LOGIN@EXAMPLE.COM ",
				Password: input.Password,
			})
			if err != nil {
				t.Fatalf("Login() error = %v", err)
			}
			if output.AccessToken == "" {
				t.Fatal("Login() returned an empty access token")
			}

			credentials, err := queries.GetUserCredentialsByEmail(ctx, input.Email)
			if err != nil {
				t.Fatalf("get credentials: %v", err)
			}
			subject, err := core.ParseTokenSubject(output.AccessToken, apitest.TestSigningKey)
			if err != nil {
				t.Fatalf("parse access token: %v", err)
			}
			if subject != credentials.UserID.String() {
				t.Fatalf("token subject = %q, want %q", subject, credentials.UserID.String())
			}
		})

		tests := []struct {
			name  string
			input auth.LoginInput
		}{
			{name: "unknown email", input: auth.LoginInput{Email: "missing@example.com", Password: input.Password}},
			{name: "wrong password", input: auth.LoginInput{Email: input.Email, Password: "wrong-password"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				output, err := service.Login(ctx, tt.input)
				if !errors.Is(err, api.ErrInvalidCredentials) {
					t.Fatalf("Login() error = %v, want %v", err, api.ErrInvalidCredentials)
				}
				if output != (auth.LoginOutput{}) {
					t.Fatalf("Login() output = %+v, want zero value", output)
				}
			})
		}
	})
}

func validRegisterInput(email, username string) auth.RegisterInput {
	return auth.RegisterInput{
		Email:     email,
		Username:  username,
		Password:  "valid-password",
		FirstName: "Test",
		LastName:  "User",
	}
}
