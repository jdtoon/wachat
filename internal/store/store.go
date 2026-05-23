// Package store owns wachat's local SQLite database: schema, prepared
// statements, and the read/write API used by the UI and the WhatsApp
// protocol layer.
//
// Memory model (see CLAUDE.md §6, §7):
//   - Media bytes never live in the DB; only file paths do.
//   - Message pages are keyset-based, never OFFSET-based.
//   - WAL + synchronous=NORMAL so a UI read can run concurrently with the
//     ingest writer.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store is wachat's local database handle. It owns the *sql.DB and is safe
// for concurrent use by multiple goroutines.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path and applies schema.sql.
// It is safe to call repeatedly against the same file — the schema is
// idempotent (PRAGMAs + CREATE TABLE IF NOT EXISTS + CREATE INDEX IF NOT
// EXISTS).
//
// The pure-Go modernc.org/sqlite driver is used so the resulting binary
// stays cgo-free (CLAUDE.md §3).
func Open(ctx context.Context, path string) (*Store, error) {
	// _pragma busy_timeout in the DSN avoids "database is locked" errors
	// when a slow writer holds the journal briefly. 5s is generous.
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %q: %w", path, err)
	}

	// Apply the schema. Pragmas are connection-scoped in SQLite, so we run
	// the whole script on a single dedicated connection and keep that
	// connection in the pool. For a personal client one connection is
	// plenty; we'll widen the pool only if we ever measure contention.
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: apply schema: %w", err)
	}

	// Schema migrations for upgraded DBs (CREATE TABLE IF NOT EXISTS
	// doesn't change existing tables).
	if err := migrateAddStatusColumn(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: migrate status: %w", err)
	}

	// Backfill the FTS5 index for any existing rows that predate the
	// virtual table (e.g. upgrading a v0.0.4 DB to v0.0.5+). Cheap when
	// already in sync — SELECT count check first to avoid the rebuild
	// on every open.
	if err := backfillFTSIfNeeded(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: backfill FTS: %w", err)
	}

	return &Store{db: db}, nil
}

// migrateAddStatusColumn adds the `status` column to `messages` on
// DBs that predate v0.1.3. Detection: PRAGMA table_info(messages) is
// scanned for the column; missing → ALTER TABLE ADD COLUMN with a
// default of 'sent' so existing rows are valid.
func migrateAddStatusColumn(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(messages)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	has := false
	for rows.Next() {
		var (
			cid         int
			name, dtype string
			notnull, pk int
			dflt        sql.NullString
		)
		if err := rows.Scan(&cid, &name, &dtype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "status" {
			has = true
			break
		}
	}
	if has {
		return nil
	}
	_, err = db.ExecContext(ctx,
		`ALTER TABLE messages ADD COLUMN status TEXT NOT NULL DEFAULT 'sent'`)
	if err != nil {
		return fmt.Errorf("alter table: %w", err)
	}
	return nil
}

// backfillFTSIfNeeded re-indexes any messages that exist in the messages
// table but not in messages_fts. Triggers handle the steady state; this
// only matters when upgrading from a wachat version that predated FTS5.
func backfillFTSIfNeeded(ctx context.Context, db *sql.DB) error {
	var pending int
	err := db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM messages m
        WHERE NOT EXISTS (SELECT 1 FROM messages_fts WHERE rowid = m.id)
    `).Scan(&pending)
	if err != nil {
		return fmt.Errorf("count missing fts rows: %w", err)
	}
	if pending == 0 {
		return nil
	}
	_, err = db.ExecContext(ctx, `
        INSERT INTO messages_fts(rowid, body)
        SELECT id, COALESCE(body, '') FROM messages
        WHERE NOT EXISTS (SELECT 1 FROM messages_fts WHERE rowid = messages.id)
    `)
	if err != nil {
		return fmt.Errorf("backfill: %w", err)
	}
	return nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB exposes the underlying *sql.DB for tests and for future package-internal
// helpers in the same module. External callers should not rely on this.
func (s *Store) DB() *sql.DB { return s.db }
