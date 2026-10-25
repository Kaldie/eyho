package services

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Keyset cursor for GET /claims: base64url("<RFC3339Nano created_at>|<claim id>"). Opaque to clients.
func encodeCursor(t time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.UTC().Format(time.RFC3339Nano) + "|" + id.String()))
}

// decodeCursor returns the first-page bounds (far future, max uuid) for an empty cursor.
func decodeCursor(c string) (time.Time, uuid.UUID, error) {
	if c == "" {
		return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"), nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok {
		return time.Time{}, uuid.Nil, errors.New("bad cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	return t, u, nil
}
