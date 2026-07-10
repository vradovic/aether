package users

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vradovic/aether/services/api/internal/db"
)

type fakeQuerier struct {
	row db.GetUserByEmailRow
}

func (f *fakeQuerier) GetUserByEmail(context.Context, string) (db.GetUserByEmailRow, error) {
	return f.row, nil
}

func TestHandlerGetUserByEmailIncludesUsername(t *testing.T) {
	querier := &fakeQuerier{row: db.GetUserByEmailRow{
		ID:        pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		Email:     "user@example.com",
		Username:  "petar",
		FirstName: "Petar",
		LastName:  "Petrović",
	}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(NewService(querier, logger), logger)
	request := httptest.NewRequest(http.MethodGet, "/users?email=user@example.com", nil)
	response := httptest.NewRecorder()

	h.getUserByEmail(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
	}
	var body userResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Username != "petar" {
		t.Errorf("username = %q, want %q", body.Username, "petar")
	}
}
