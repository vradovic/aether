package core

import (
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrInvalidID = errors.New("invalid id")

func ParseUUID(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return pgtype.UUID{}, ErrInvalidID
	}
	return id, nil
}
