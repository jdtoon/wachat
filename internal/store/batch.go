package store

import (
	"context"
	"fmt"
)

// InsertBatch persists many messages and upserts the affected chats'
// last_ts in a single transaction. Used by the history-sync path which
// can deliver thousands of messages in one whatsmeow event — per-row
// Insert calls would each take a write lock and be ~10× slower.
//
// Idempotent on wa_id (same ON CONFLICT DO NOTHING contract as Insert).
// Returns the number of newly-created rows; redeliveries are reported as
// not-created.
//
// unread is never bumped here — history sync is replay of messages
// already seen on the phone; only LIVE events.Message paths
// (handler.OnMessage) increment the unread counter.
func (s *Store) InsertBatch(ctx context.Context, msgs []Message) (int, error) {
	if len(msgs) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store.InsertBatch: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	insertMsg, err := tx.PrepareContext(ctx, `
        INSERT INTO messages (
            wa_id, chat_jid, sender_jid, ts, body,
            media_path, media_type, status,
            quoted_waid, quoted_body, quoted_sender, edited, revoked,
            link_url, link_title, link_desc, starred
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(wa_id) DO NOTHING
    `)
	if err != nil {
		return 0, fmt.Errorf("store.InsertBatch: prepare insert: %w", err)
	}
	defer insertMsg.Close()

	chatLastTS := make(map[string]int64)
	created := 0
	for _, m := range msgs {
		if m.WAID == "" || m.ChatJID == "" {
			continue
		}
		status := m.Status
		if status == "" {
			status = StatusSent
		}
		res, err := insertMsg.ExecContext(ctx,
			m.WAID, m.ChatJID, nullableString(m.SenderJID), m.TS,
			nullableString(m.Body), nullableString(m.MediaPath), nullableString(m.MediaType),
			status,
			nullableString(m.QuotedWAID), nullableString(m.QuotedBody), nullableString(m.QuotedSender),
			boolToInt(m.Edited), boolToInt(m.Revoked),
			nullableString(m.LinkURL), nullableString(m.LinkTitle), nullableString(m.LinkDesc),
			boolToInt(m.Starred),
		)
		if err != nil {
			return 0, fmt.Errorf("store.InsertBatch: insert %s: %w", m.WAID, err)
		}
		n, _ := res.RowsAffected()
		created += int(n)
		if ts, ok := chatLastTS[m.ChatJID]; !ok || m.TS > ts {
			chatLastTS[m.ChatJID] = m.TS
		}
	}

	chatUpsert, err := tx.PrepareContext(ctx, `
        INSERT INTO chats (jid, name, last_ts, unread)
        VALUES (?, '', ?, 0)
        ON CONFLICT(jid) DO UPDATE SET
            last_ts = MAX(COALESCE(last_ts, 0), excluded.last_ts)
    `)
	if err != nil {
		return 0, fmt.Errorf("store.InsertBatch: prepare chat upsert: %w", err)
	}
	defer chatUpsert.Close()
	for jid, ts := range chatLastTS {
		if _, err := chatUpsert.ExecContext(ctx, jid, ts); err != nil {
			return 0, fmt.Errorf("store.InsertBatch: chat upsert %s: %w", jid, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("store.InsertBatch: commit: %w", err)
	}
	return created, nil
}
