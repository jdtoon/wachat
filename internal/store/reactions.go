package store

import (
	"context"
	"fmt"
)

// Reaction is one emoji reaction on a message. target_waid identifies
// the message being reacted to; sender_jid is who reacted.
type Reaction struct {
	TargetWAID string
	SenderJID  string
	Emoji      string
	TS         int64
}

// SetReaction upserts a reaction. emoji == "" removes the reaction
// (matches WhatsApp's wire semantics: an empty ReactionMessage.Text
// is the "undo" signal).
//
// Caller is the wa.Handler when an inbound events.Message carries a
// ReactionMessage proto, and main.go's reaction-send path.
func (s *Store) SetReaction(ctx context.Context, targetWAID, senderJID, emoji string, ts int64) error {
	if targetWAID == "" || senderJID == "" {
		return fmt.Errorf("store.SetReaction: target and sender required")
	}
	if emoji == "" {
		_, err := s.db.ExecContext(ctx,
			`DELETE FROM reactions WHERE target_waid = ? AND sender_jid = ?`,
			targetWAID, senderJID)
		if err != nil {
			return fmt.Errorf("store.SetReaction: delete: %w", err)
		}
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO reactions (target_waid, sender_jid, emoji, ts)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(target_waid, sender_jid) DO UPDATE SET
            emoji = excluded.emoji,
            ts    = excluded.ts
    `, targetWAID, senderJID, emoji, ts)
	if err != nil {
		return fmt.Errorf("store.SetReaction: upsert: %w", err)
	}
	return nil
}

// ListReactions returns every reaction on a message, oldest-first.
func (s *Store) ListReactions(ctx context.Context, targetWAID string) ([]Reaction, error) {
	if targetWAID == "" {
		return nil, fmt.Errorf("store.ListReactions: targetWAID required")
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT target_waid, sender_jid, emoji, ts
        FROM reactions
        WHERE target_waid = ?
        ORDER BY ts ASC
    `, targetWAID)
	if err != nil {
		return nil, fmt.Errorf("store.ListReactions: %w", err)
	}
	defer rows.Close()
	var out []Reaction
	for rows.Next() {
		var r Reaction
		if err := rows.Scan(&r.TargetWAID, &r.SenderJID, &r.Emoji, &r.TS); err != nil {
			return nil, fmt.Errorf("store.ListReactions: scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReactionsForChat fetches every reaction across the loaded message
// window in one query, indexed by target_waid for cheap per-bubble
// lookup. Returns nil if there are no waIDs.
//
// Used by the UI on a chat select / page-extend so the bubble
// rendering can render reaction chips without per-message DB calls.
func (s *Store) ReactionsForChat(ctx context.Context, targetWAIDs []string) (map[string][]Reaction, error) {
	if len(targetWAIDs) == 0 {
		return nil, nil
	}
	// Build (?, ?, ?, ...) for the IN clause. Bounded by viewport
	// size — typically ≤ 50 ids.
	placeholders := make([]byte, 0, len(targetWAIDs)*2)
	args := make([]any, 0, len(targetWAIDs))
	for i, id := range targetWAIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT target_waid, sender_jid, emoji, ts
        FROM reactions
        WHERE target_waid IN (`+string(placeholders)+`)
        ORDER BY ts ASC
    `, args...)
	if err != nil {
		return nil, fmt.Errorf("store.ReactionsForChat: %w", err)
	}
	defer rows.Close()
	out := make(map[string][]Reaction)
	for rows.Next() {
		var r Reaction
		if err := rows.Scan(&r.TargetWAID, &r.SenderJID, &r.Emoji, &r.TS); err != nil {
			return nil, fmt.Errorf("store.ReactionsForChat: scan: %w", err)
		}
		out[r.TargetWAID] = append(out[r.TargetWAID], r)
	}
	return out, rows.Err()
}
