package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Settings is a tiny key/value table for UI preferences (theme,
// density, sidebar width, ...). Lives alongside the messages so a
// fresh DB starts with sensible defaults and a paired DB carries the
// user's choices across launches.
//
// The schema lives in schema.sql; this file is just the read/write
// helpers.

// GetSetting returns the value for key, or "" if the key doesn't
// exist. Errors only on database failure, not on missing key — UI
// code can just call GetSetting and treat "" as "default".
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("store.GetSetting: key is required")
	}
	var value string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM settings WHERE key = ?`, key,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("store.GetSetting: %w", err)
	}
	return value, nil
}

// SetSetting upserts a key/value pair. Used for the dark-mode toggle,
// density preference, sidebar width, etc.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	if key == "" {
		return fmt.Errorf("store.SetSetting: key is required")
	}
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO settings (key, value) VALUES (?, ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value
    `, key, value)
	if err != nil {
		return fmt.Errorf("store.SetSetting: %w", err)
	}
	return nil
}
