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
	const selectCols = `SELECT id, wa_id, chat_jid, sender_jid, ts, body, media_path, media_type FROM messages`
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
		if err := rows.Scan(&m.ID, &waID, &m.ChatJID, &senderJID, &m.TS, &body, &mediaPath, &mediaType); err != nil {
			return nil, Cursor{}, fmt.Errorf("store.PageOlder: scan: %w", err)
		}
		m.WAID = waID.String
		m.SenderJID = senderJID.String
		m.Body = body.String
		m.MediaPath = mediaPath.String
		m.MediaType = mediaType.String
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
