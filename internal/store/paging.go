package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Cursor is the keyset position used by PageOlder. It is a composite of
// (TS, ID) so that messages sharing a millisecond are neither skipped nor
// duplicated across page boundaries.
//
// The zero value means "start from the newest message" — pass Cursor{} as
// the `before` argument for the first page.
type Cursor struct {
	TS int64
	ID int64
}

// IsZero reports whether c is the start-of-history cursor.
func (c Cursor) IsZero() bool { return c.TS == 0 && c.ID == 0 }

// PageAround returns a window of messages centered on anchorID — `before`
// older messages and `after` newer messages on the other side, plus the
// anchor itself. All in newest-first order so the caller can plug the
// result straight into State.Messages.
//
// Keyset-style (no OFFSET): both halves use bounded WHERE clauses with
// the index on (chat_jid, ts DESC). The anchor row is fetched
// separately and inserted at the head of the older half so the order
// matches state.Messages (newest-first).
//
// Used by ui.JumpToMessage to land directly on a search hit's
// conversation page without keyset-paging from the newest end.
func (s *Store) PageAround(ctx context.Context, chatJID string, anchorID int64, before, after int) ([]Message, Cursor, error) {
	if chatJID == "" {
		return nil, Cursor{}, fmt.Errorf("store.PageAround: chatJID is required")
	}
	if before < 0 || after < 0 {
		return nil, Cursor{}, fmt.Errorf("store.PageAround: before/after must be non-negative")
	}

	const selectCols = `SELECT id, wa_id, chat_jid, sender_jid, ts, body, media_path, media_type, status FROM messages`

	// Anchor row — also gives us the TS we need for the OLDER side.
	var anchor Message
	{
		var waID, senderJID, body, mediaPath, mediaType, status sql.NullString
		err := s.db.QueryRowContext(ctx, selectCols+` WHERE id = ? AND chat_jid = ?`, anchorID, chatJID).
			Scan(&anchor.ID, &waID, &anchor.ChatJID, &senderJID, &anchor.TS, &body, &mediaPath, &mediaType, &status)
		if err != nil {
			return nil, Cursor{}, fmt.Errorf("store.PageAround: anchor %d: %w", anchorID, err)
		}
		anchor.WAID = waID.String
		anchor.SenderJID = senderJID.String
		anchor.Body = body.String
		anchor.MediaPath = mediaPath.String
		anchor.MediaType = mediaType.String
		anchor.Status = status.String
	}

	// NEWER half: messages strictly newer than the anchor. ORDER BY ts
	// DESC, id DESC so the first row of this slice is the newest in
	// the window.
	newer, err := s.queryMessages(ctx, selectCols+`
        WHERE chat_jid = ? AND (ts > ? OR (ts = ? AND id > ?))
        ORDER BY ts DESC, id DESC
        LIMIT ?
    `, chatJID, anchor.TS, anchor.TS, anchor.ID, after)
	if err != nil {
		return nil, Cursor{}, fmt.Errorf("store.PageAround: newer: %w", err)
	}

	// OLDER half: messages strictly older than the anchor.
	older, err := s.queryMessages(ctx, selectCols+`
        WHERE chat_jid = ? AND (ts < ? OR (ts = ? AND id < ?))
        ORDER BY ts DESC, id DESC
        LIMIT ?
    `, chatJID, anchor.TS, anchor.TS, anchor.ID, before)
	if err != nil {
		return nil, Cursor{}, fmt.Errorf("store.PageAround: older: %w", err)
	}

	out := make([]Message, 0, len(newer)+1+len(older))
	out = append(out, newer...)
	out = append(out, anchor)
	out = append(out, older...)

	next := Cursor{}
	if n := len(out); n > 0 {
		next = Cursor{TS: out[n-1].TS, ID: out[n-1].ID}
	}
	return out, next, nil
}

// queryMessages is a small helper for the two paging queries.
func (s *Store) queryMessages(ctx context.Context, q string, args ...any) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var waID, senderJID, body, mediaPath, mediaType, status sql.NullString
		if err := rows.Scan(&m.ID, &waID, &m.ChatJID, &senderJID, &m.TS, &body, &mediaPath, &mediaType, &status); err != nil {
			return nil, err
		}
		m.WAID = waID.String
		m.SenderJID = senderJID.String
		m.Body = body.String
		m.MediaPath = mediaPath.String
		m.MediaType = mediaType.String
		m.Status = status.String
		out = append(out, m)
	}
	return out, rows.Err()
}

// PageOlder returns up to limit messages from chatJID strictly older than
// `before`, ordered newest-first. The returned `next` cursor is the
// (TS, ID) of the oldest row in the page — pass it as `before` to fetch
// the next page.
//
// On the first call, pass Cursor{} (zero value) to get the most recent
// messages. When the returned slice is shorter than limit, the history is
// exhausted; calling again with the returned next cursor returns an empty
// slice.
//
// The query is keyset-based and uses idx_chat_ts (chat_jid, ts DESC), so
// latency is O(page) regardless of how deep we have paged. We never use
// OFFSET — see CLAUDE.md §6.
func (s *Store) PageOlder(ctx context.Context, chatJID string, before Cursor, limit int) ([]Message, Cursor, error) {
	if chatJID == "" {
		return nil, Cursor{}, fmt.Errorf("store.PageOlder: chatJID is required")
	}
	if limit <= 0 {
		return nil, Cursor{}, fmt.Errorf("store.PageOlder: limit must be positive, got %d", limit)
	}

	var (
		rows *sql.Rows
		err  error
	)
	const selectCols = `SELECT id, wa_id, chat_jid, sender_jid, ts, body, media_path, media_type, status FROM messages`
	if before.IsZero() {
		rows, err = s.db.QueryContext(ctx, selectCols+`
            WHERE chat_jid = ?
            ORDER BY ts DESC, id DESC
            LIMIT ?
        `, chatJID, limit)
	} else {
		// (ts < before.TS) OR (ts = before.TS AND id < before.ID) — the
		// canonical "tuple less-than" expressed in SQL that does not need
		// row-value support.
		rows, err = s.db.QueryContext(ctx, selectCols+`
            WHERE chat_jid = ?
              AND (ts < ? OR (ts = ? AND id < ?))
            ORDER BY ts DESC, id DESC
            LIMIT ?
        `, chatJID, before.TS, before.TS, before.ID, limit)
	}
	if err != nil {
		return nil, Cursor{}, fmt.Errorf("store.PageOlder: query: %w", err)
	}
	defer rows.Close()

	out := make([]Message, 0, limit)
	for rows.Next() {
		var m Message
		var waID, senderJID, body, mediaPath, mediaType sql.NullString
		var status sql.NullString
		if err := rows.Scan(&m.ID, &waID, &m.ChatJID, &senderJID, &m.TS, &body, &mediaPath, &mediaType, &status); err != nil {
			return nil, Cursor{}, fmt.Errorf("store.PageOlder: scan: %w", err)
		}
		m.WAID = waID.String
		m.SenderJID = senderJID.String
		m.Body = body.String
		m.MediaPath = mediaPath.String
		m.MediaType = mediaType.String
		m.Status = status.String
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, Cursor{}, fmt.Errorf("store.PageOlder: rows: %w", err)
	}

	next := Cursor{}
	if n := len(out); n > 0 {
		next = Cursor{TS: out[n-1].TS, ID: out[n-1].ID}
	}
	return out, next, nil
}
