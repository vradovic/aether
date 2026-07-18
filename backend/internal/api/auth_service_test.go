package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/vradovic/aether/services/api/internal/core"
	"github.com/vradovic/aether/services/api/internal/db"
)

const (
	testSigningKey = "integration-test-signing-key"
)

func TestAuthService(t *testing.T) {
	ctx := context.Background()
	conn := startAuthTestDatabase(t, ctx)
	queries := db.New(conn)
	service := NewAuthService(queries, testSigningKey)

	t.Run("register", func(t *testing.T) {
		t.Run("stores normalized user and hashed password", func(t *testing.T) {
			input := RegisterInput{
				email:     "  Alice.Example@EXAMPLE.COM ",
				username:  "  alice_example  ",
				password:  "correct-horse-battery-staple",
				firstName: "  Alice  ",
				lastName:  "  Example  ",
			}

			if err := service.Register(ctx, input); err != nil {
				t.Fatalf("Register() error = %v", err)
			}

			var email, username, passwordHash, firstName, lastName string
			err := conn.QueryRow(ctx, `
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
			if passwordHash == input.password {
				t.Fatal("password was stored as plaintext")
			}
			if err := verifyPassword(input.password, passwordHash); err != nil {
				t.Fatalf("stored password hash does not match password: %v", err)
			}
		})

		t.Run("rejects invalid input without writing", func(t *testing.T) {
			tests := []struct {
				name   string
				mutate func(*RegisterInput)
				want   error
			}{
				{name: "invalid email", mutate: func(i *RegisterInput) { i.email = "not-an-email" }, want: errEmailFormat},
				{name: "short password", mutate: func(i *RegisterInput) { i.password = "short" }, want: errPasswordLength},
				{name: "long password", mutate: func(i *RegisterInput) { i.password = strings.Repeat("a", maxPasswordLengthBytes+1) }, want: errPasswordLength},
				{name: "short username", mutate: func(i *RegisterInput) { i.username = "ab" }, want: errUsernameLength},
				{name: "long username", mutate: func(i *RegisterInput) { i.username = strings.Repeat("a", maxUsernameLength+1) }, want: errUsernameLength},
				{name: "short first name", mutate: func(i *RegisterInput) { i.firstName = "A" }, want: errNameLength},
				{name: "long last name", mutate: func(i *RegisterInput) { i.lastName = strings.Repeat("a", maxNameLength+1) }, want: errNameLength},
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
					if err := conn.QueryRow(ctx, "SELECT count(*) FROM users WHERE email = $1", input.normalize().email).Scan(&count); err != nil {
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
				input      RegisterInput
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
			output, err := service.Login(ctx, LoginInput{
				Email:    "  LOGIN@EXAMPLE.COM ",
				Password: input.password,
			})
			if err != nil {
				t.Fatalf("Login() error = %v", err)
			}
			if output.AccessToken == "" {
				t.Fatal("Login() returned an empty access token")
			}

			credentials, err := queries.GetUserCredentialsByEmail(ctx, input.email)
			if err != nil {
				t.Fatalf("get credentials: %v", err)
			}
			subject, err := core.ParseTokenSubject(output.AccessToken, testSigningKey)
			if err != nil {
				t.Fatalf("parse access token: %v", err)
			}
			if subject != credentials.UserID.String() {
				t.Fatalf("token subject = %q, want %q", subject, credentials.UserID.String())
			}
		})

		tests := []struct {
			name  string
			input LoginInput
		}{
			{name: "unknown email", input: LoginInput{Email: "missing@example.com", Password: input.password}},
			{name: "wrong password", input: LoginInput{Email: input.email, Password: "wrong-password"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				output, err := service.Login(ctx, tt.input)
				if !errors.Is(err, ErrInvalidCredentials) {
					t.Fatalf("Login() error = %v, want %v", err, ErrInvalidCredentials)
				}
				if output != (LoginOutput{}) {
					t.Fatalf("Login() output = %+v, want zero value", output)
				}
			})
		}
	})
}

func validRegisterInput(email, username string) RegisterInput {
	return RegisterInput{
		email:     email,
		username:  username,
		password:  "valid-password",
		firstName: "Test",
		lastName:  "User",
	}
}

func startAuthTestDatabase(t *testing.T, ctx context.Context) *pgx.Conn {
	t.Helper()

	container, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("aether_test"),
		postgres.WithUsername("aether"),
		postgres.WithPassword("aether"),
		postgres.BasicWaitStrategies(),
	)
	if container != nil {
		t.Cleanup(func() {
			if err := container.Terminate(context.Background()); err != nil {
				t.Errorf("terminate PostgreSQL container: %v", err)
			}
		})
	}
	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}

	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get PostgreSQL connection string: %v", err)
	}
	conn, err := pgx.Connect(ctx, connectionString)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Errorf("close PostgreSQL connection: %v", err)
		}
	})

	migrationPath := filepath.Join("..", "..", "sql", "migrations", "20260709223158_create_users_table.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read users migration: %v", err)
	}
	upMigration, _, found := strings.Cut(string(migration), "-- +goose Down")
	if !found {
		t.Fatalf("users migration %s has no Goose down marker", migrationPath)
	}
	if _, err := conn.Exec(ctx, upMigration); err != nil {
		t.Fatalf("apply users migration: %v", err)
	}

	return conn
}
