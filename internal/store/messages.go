package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Message is a single WhatsApp message as wachat persists it.
//
// WAID is whatsmeow's per-message identifier and is the dedup key — see
// CLAUDE.md §7. TS is unix-milliseconds. MediaPath, if non-empty, points to
// a file on disk in the media/ directory; bytes are never stored in the DB.
//
// ID is the SQLite rowid. It is populated by reads and ignored by Insert
// (the database assigns it). The (TS, ID) pair is the keyset cursor used
// by PageOlder to avoid the duplicate / skip hazard when two messages
// share a millisecond.
type Message struct {
	ID        int64
	WAID      string
	ChatJID   string
	SenderJID string
	TS        int64
	Body      string
	MediaPath string
	MediaType string
}

// Insert persists m and updates the chat's last_ts atomically.
//
// Returns created=true when a new row was written, false when the message
// was already present (dedup hit on wa_id). Insert is idempotent: a
// redelivered message neither duplicates the row nor double-bumps unread.
//
// If created is true and bumpUnread is true, chats.unread is incremented by
// one. The caller decides this — typically pass true for incoming messages
// and false for ones sent by us.
//
// The chat row is created on demand with an empty name if needed; call
// UpsertChat separately to set or update the display name.
func (s *Store) Insert(ctx context.Context, m Message, bumpUnread bool) (bool, error) {
	if m.WAID == "" {
		return false, fmt.Errorf("store.Insert: WAID is required")
	}
	if m.ChatJID == "" {
		return false, fmt.Errorf("store.Insert: ChatJID is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store.Insert: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // safe no-op after Commit

	// INSERT OR IGNORE returns rowsAffected=1 on insert and 0 on dedup hit,
	// which is exactly the signal we want without a separate SELECT.
	res, err := tx.ExecContext(ctx, `
        INSERT INTO messages (wa_id, chat_jid, sender_jid, ts, body, media_path, media_type)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(wa_id) DO NOTHING
    `,
		m.WAID, m.ChatJID, nullableString(m.SenderJID), m.TS,
		nullableString(m.Body), nullableString(m.MediaPath), nullableString(m.MediaType),
	)
	if err != nil {
		return false, fmt.Errorf("store.Insert: insert message: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store.Insert: rows affected: %w", err)
	}
	created := affected == 1

	// Update the chat's last_ts only if this message is newer than what's
	// already recorded. Avoids out-of-order arrivals overwriting a newer
	// timestamp. The COALESCE handles the row-doesn't-exist-yet case.
	unreadDelta := 0
	if created && bumpUnread {
		unreadDelta = 1
	}
	_, err = tx.ExecContext(ctx, `
        INSERT INTO chats (jid, name, last_ts, unread)
        VALUES (?, '', ?, ?)
        ON CONFLICT(jid) DO UPDATE SET
            last_ts = MAX(COALESCE(last_ts, 0), excluded.last_ts),
            unread  = unread + ?
    `,
		m.ChatJID, m.TS, unreadDelta, unreadDelta,
	)
	if err != nil {
		return false, fmt.Errorf("store.Insert: upsert chat row: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store.Insert: commit: %w", err)
	}
	return created, nil
}

// UpsertChat sets a chat's display name, creating the chat row if it does
// not exist yet. It does not modify last_ts or unread — those are owned by
// Insert and MarkRead.
func (s *Store) UpsertChat(ctx context.Context, jid, name string) error {
	if jid == "" {
		return fmt.Errorf("store.UpsertChat: jid is required")
	}
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO chats (jid, name, last_ts, unread)
        VALUES (?, ?, NULL, 0)
        ON CONFLICT(jid) DO UPDATE SET name = excluded.name
    `, jid, name)
	if err != nil {
		return fmt.Errorf("store.UpsertChat: %w", err)
	}
	return nil
}

// MarkRead clears the unread counter for a chat. No-op if the chat does
// not exist.
func (s *Store) MarkRead(ctx context.Context, jid string) error {
	if jid == "" {
		return fmt.Errorf("store.MarkRead: jid is required")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET unread = 0 WHERE jid = ?`, jid)
	if err != nil {
		return fmt.Errorf("store.MarkRead: %w", err)
	}
	return nil
}

// nullableString returns sql.NullString{Valid: false} for empty strings so
// the column stores NULL rather than an empty string. This keeps "text-only
// message" (media_path IS NULL) distinguishable from "explicitly empty path".
func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
