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
//
// Status tracks the delivery state for outgoing messages (and stays
// "sent" for incoming). Transitions:
//
//	pending → sent      (server ack from whatsmeow.SendMessage)
//	sent    → delivered (events.Receipt with ReceiptTypeDelivered)
//	delivered → read    (events.Receipt with ReceiptTypeRead)
//	(voice) sent → played (ReceiptTypePlayed; future use)
//
// Empty string defaults to StatusSent on Insert.
type Message struct {
	ID        int64
	WAID      string
	ChatJID   string
	SenderJID string
	TS        int64
	Body      string
	MediaPath string
	MediaType string
	Status    string

	// Quoted reply info — empty for non-reply messages.
	QuotedWAID   string
	QuotedBody   string
	QuotedSender string

	// Edited: sender used WhatsApp's "edit message" feature.
	Edited bool
	// Revoked: sender used "delete for everyone." Body should be
	// rendered as a placeholder rather than the stored content.
	Revoked bool

	// Link preview info — populated when the message had an
	// ExtendedTextMessage with a URL preview. The UI renders these
	// as a small card below the body.
	LinkURL   string
	LinkTitle string
	LinkDesc  string
}

// Message status constants. Keep these in sync with bubble.go's tick
// rendering and the receipt-handling switch in wa/handler.go.
const (
	StatusPending   = "pending"
	StatusSent      = "sent"
	StatusDelivered = "delivered"
	StatusRead      = "read"
	StatusPlayed    = "played"
)

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

	status := m.Status
	if status == "" {
		status = StatusSent
	}

	// INSERT OR IGNORE returns rowsAffected=1 on insert and 0 on dedup hit,
	// which is exactly the signal we want without a separate SELECT.
	res, err := tx.ExecContext(ctx, `
        INSERT INTO messages (
            wa_id, chat_jid, sender_jid, ts, body,
            media_path, media_type, status,
            quoted_waid, quoted_body, quoted_sender, edited, revoked,
            link_url, link_title, link_desc
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(wa_id) DO NOTHING
    `,
		m.WAID, m.ChatJID, nullableString(m.SenderJID), m.TS,
		nullableString(m.Body), nullableString(m.MediaPath), nullableString(m.MediaType),
		status,
		nullableString(m.QuotedWAID), nullableString(m.QuotedBody), nullableString(m.QuotedSender),
		boolToInt(m.Edited), boolToInt(m.Revoked),
		nullableString(m.LinkURL), nullableString(m.LinkTitle), nullableString(m.LinkDesc),
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

// ApplyEdit updates an existing message's body and flips the edited
// flag. Called from wa.Handler when a ProtocolMessage MESSAGE_EDIT
// arrives. No-op for unknown wa_id (the edit might land before the
// original via out-of-order delivery).
func (s *Store) ApplyEdit(ctx context.Context, waID, newBody string) error {
	if waID == "" {
		return fmt.Errorf("store.ApplyEdit: waID is required")
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE messages SET body = ?, edited = 1 WHERE wa_id = ?`,
		nullableString(newBody), waID)
	if err != nil {
		return fmt.Errorf("store.ApplyEdit: %w", err)
	}
	return nil
}

// ApplyRevoke flips the revoked flag for the message. Called from
// wa.Handler when a ProtocolMessage REVOKE arrives. The body is
// preserved on disk (the user can still see what was said, if we
// chose to surface it); the UI renders a placeholder when revoked=1.
func (s *Store) ApplyRevoke(ctx context.Context, waID string) error {
	if waID == "" {
		return fmt.Errorf("store.ApplyRevoke: waID is required")
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE messages SET revoked = 1 WHERE wa_id = ?`, waID)
	if err != nil {
		return fmt.Errorf("store.ApplyRevoke: %w", err)
	}
	return nil
}

// boolToInt: SQLite has no real bool; we store 0/1 in INTEGER columns.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// UpdateStatus sets messages.status by wa_id. Status transitions are
// driven by wa.Handler from events.Receipt; this is the single mutating
// API the wa layer calls.
//
// No-op for an unknown wa_id (defensive: a receipt may arrive for a
// message we never persisted, e.g. very old history not synced yet).
func (s *Store) UpdateStatus(ctx context.Context, waID, status string) error {
	if waID == "" {
		return fmt.Errorf("store.UpdateStatus: waID is required")
	}
	if status == "" {
		return fmt.Errorf("store.UpdateStatus: status is required")
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE messages SET status = ? WHERE wa_id = ?`, status, waID)
	if err != nil {
		return fmt.Errorf("store.UpdateStatus: %w", err)
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
