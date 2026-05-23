package store

import (
	"context"
	"fmt"
)

// SetPinned flips the chat's pinned flag. Used by wa.Handler when
// events.Pin arrives. Pinned chats sort to the top of the chat list.
func (s *Store) SetPinned(ctx context.Context, jid string, pinned bool) error {
	if jid == "" {
		return fmt.Errorf("store.SetPinned: jid required")
	}
	if err := s.ensureChatRow(ctx, jid); err != nil {
		return err
	}
	v := 0
	if pinned {
		v = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE chats SET pinned = ? WHERE jid = ?`, v, jid)
	if err != nil {
		return fmt.Errorf("store.SetPinned: %w", err)
	}
	return nil
}

// SetArchived flips the chat's archived flag.
func (s *Store) SetArchived(ctx context.Context, jid string, archived bool) error {
	if jid == "" {
		return fmt.Errorf("store.SetArchived: jid required")
	}
	if err := s.ensureChatRow(ctx, jid); err != nil {
		return err
	}
	v := 0
	if archived {
		v = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE chats SET archived = ? WHERE jid = ?`, v, jid)
	if err != nil {
		return fmt.Errorf("store.SetArchived: %w", err)
	}
	return nil
}

// SetMuteUntil records the mute deadline (unix millis); 0 unmutes,
// -1 mutes forever.
func (s *Store) SetMuteUntil(ctx context.Context, jid string, muteUntilMS int64) error {
	if jid == "" {
		return fmt.Errorf("store.SetMuteUntil: jid required")
	}
	if err := s.ensureChatRow(ctx, jid); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE chats SET mute_until = ? WHERE jid = ?`, muteUntilMS, jid)
	if err != nil {
		return fmt.Errorf("store.SetMuteUntil: %w", err)
	}
	return nil
}

// ensureChatRow inserts a placeholder chat row if none exists. App-
// state events can arrive before any message in that chat (e.g.
// "pin this chat I just created on my phone"), and we need a row to
// update.
func (s *Store) ensureChatRow(ctx context.Context, jid string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO chats (jid, name, last_ts, unread) VALUES (?, '', NULL, 0) ON CONFLICT(jid) DO NOTHING`, jid)
	return err
}
