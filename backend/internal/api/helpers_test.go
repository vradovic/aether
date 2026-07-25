package api_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/vradovic/aether/backend/internal/db"
)

func startDatabase(t *testing.T, ctx context.Context) (*pgxpool.Pool, *db.Queries) {
	t.Helper()

	container, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("password"),
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

	sqlDB, err := sql.Open("pgx", connectionString)
	if err != nil {
		t.Fatalf("sql open error: %w", err)
	}
	defer sqlDB.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose set dialect error: %w", err)
	}

	migrationsDir := filepath.Join("..", "..", "sql", "migrations")
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		t.Fatalf("goose up error: %w", err)
	}

	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	queries := db.New(pool)

	return pool, queries
}
