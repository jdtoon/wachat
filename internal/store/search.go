package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SearchHit is one match from a full-text search. Snippet contains the
// matched text with markup tokens around the matching term — we use
// "[[" / "]]" so the UI can render the highlighted span in the accent
// color without HTML escaping concerns. ChatJID lets the UI jump
// straight to the conversation, MessageID into the right page via
// PageAround.
type SearchHit struct {
	ChatJID   string
	ChatName  string
	MessageID int64
	WAID      string
	TS        int64
	SenderJID string
	Snippet   string
}

// Search runs an FTS5 query over the message body and returns up to
// limit hits sorted by FTS5's relevance ranking (newest tiebreaker).
//
// query is passed as-is to FTS5's MATCH operator — callers should sanitize
// double-quotes if they accept raw user input that may contain them.
// For wachat the input is the user's literal search string; we wrap it
// in double quotes and replace internal double quotes with single ones
// so any string is a valid FTS5 phrase query.
//
// Results join the chats table to surface the chat name so the UI can
// render "Alice — yesterday — '… hi __there__ …'" without an extra
// per-hit lookup.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		return nil, fmt.Errorf("store.Search: limit must be positive, got %d", limit)
	}

	rows, err := s.db.QueryContext(ctx, `
        SELECT
            m.id, m.wa_id, m.chat_jid, COALESCE(c.name, ''), m.ts,
            COALESCE(m.sender_jid, ''),
            snippet(messages_fts, 0, '[[', ']]', '…', 16) AS snip
        FROM messages_fts
        JOIN messages m ON m.id = messages_fts.rowid
        LEFT JOIN chats c ON c.jid = m.chat_jid
        WHERE messages_fts MATCH ?
        ORDER BY rank, m.ts DESC
        LIMIT ?
    `, ftsPhrase(query), limit)
	if err != nil {
		return nil, fmt.Errorf("store.Search: query: %w", err)
	}
	defer rows.Close()

	out := make([]SearchHit, 0, limit)
	for rows.Next() {
		var h SearchHit
		var waID sql.NullString
		if err := rows.Scan(&h.MessageID, &waID, &h.ChatJID, &h.ChatName, &h.TS, &h.SenderJID, &h.Snippet); err != nil {
			return nil, fmt.Errorf("store.Search: scan: %w", err)
		}
		h.WAID = waID.String
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store.Search: rows: %w", err)
	}
	return out, nil
}

// ftsPhrase wraps the user's query so it's safe as an FTS5 phrase
// expression. Strategy: replace internal double quotes with single
// quotes (FTS5 doesn't escape ""), then wrap the whole thing in double
// quotes so any tokens (including operators like AND/OR) are treated
// as literal text rather than FTS5 syntax.
//
// Exposed at package level so search_test.go can exercise it directly.
func ftsPhrase(q string) string {
	return `"` + strings.ReplaceAll(q, `"`, `'`) + `"`
}
