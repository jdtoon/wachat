package store

import (
	"context"
	"database/sql"
	"testing"
)

func msg(waID, chatJID string, ts int64) Message {
	return Message{
		WAID:      waID,
		ChatJID:   chatJID,
		SenderJID: "alice@s.whatsapp.net",
		TS:        ts,
		Body:      "hello " + waID,
	}
}

func chatRow(t *testing.T, s *Store, jid string) (name string, lastTS int64, unread int) {
	t.Helper()
	var n string
	var lt sql.NullInt64
	err := s.DB().QueryRow(
		"SELECT name, last_ts, unread FROM chats WHERE jid=?", jid,
	).Scan(&n, &lt, &unread)
	if err != nil {
		t.Fatalf("chatRow %q: %v", jid, err)
	}
	if lt.Valid {
		lastTS = lt.Int64
	}
	return n, lastTS, unread
}

func countMessages(t *testing.T, s *Store, chatJID string) int {
	t.Helper()
	var n int
	if err := s.DB().QueryRow(
		"SELECT COUNT(*) FROM messages WHERE chat_jid=?", chatJID,
	).Scan(&n); err != nil {
		t.Fatalf("countMessages: %v", err)
	}
	return n
}

func TestInsert_CreatesRowAndChat(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	created, err := s.Insert(ctx, msg("w1", "c1", 1000), true)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if !created {
		t.Errorf("created = false, want true for first insert")
	}
	if n := countMessages(t, s, "c1"); n != 1 {
		t.Errorf("message count = %d, want 1", n)
	}

	_, ts, unread := chatRow(t, s, "c1")
	if ts != 1000 {
		t.Errorf("chat last_ts = %d, want 1000", ts)
	}
	if unread != 1 {
		t.Errorf("chat unread = %d, want 1", unread)
	}
}

func TestInsert_DedupsOnWAID(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	c1, _ := s.Insert(ctx, msg("w1", "c1", 1000), true)
	c2, _ := s.Insert(ctx, msg("w1", "c1", 1000), true)

	if !c1 {
		t.Errorf("first insert: created = false")
	}
	if c2 {
		t.Errorf("second insert (dup wa_id): created = true, want false")
	}
	if n := countMessages(t, s, "c1"); n != 1 {
		t.Errorf("message count = %d, want 1 (dedup)", n)
	}
	if _, _, unread := chatRow(t, s, "c1"); unread != 1 {
		t.Errorf("unread = %d, want 1 (must not double-bump on dedup)", unread)
	}
}

func TestInsert_BumpUnreadFalse(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	if _, err := s.Insert(context.Background(), msg("w1", "c1", 1000), false); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	_ = ctx
	if _, _, unread := chatRow(t, s, "c1"); unread != 0 {
		t.Errorf("unread = %d, want 0 when bumpUnread=false", unread)
	}
}

func TestInsert_LastTSIsMax(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	// Insert newer first, then older. last_ts must stay at the newer value.
	mustInsert(t, s, ctx, msg("w2", "c1", 2000), true)
	mustInsert(t, s, ctx, msg("w1", "c1", 1000), true)

	if _, ts, _ := chatRow(t, s, "c1"); ts != 2000 {
		t.Errorf("last_ts = %d, want 2000 (max of inserts)", ts)
	}
}

func TestInsert_RequiresWAID(t *testing.T) {
	s := openTempStore(t)

	if _, err := s.Insert(context.Background(), Message{ChatJID: "c1", TS: 1}, false); err == nil {
		t.Error("Insert with empty WAID: want error, got nil")
	}
}

func TestInsert_RequiresChatJID(t *testing.T) {
	s := openTempStore(t)

	if _, err := s.Insert(context.Background(), Message{WAID: "w1", TS: 1}, false); err == nil {
		t.Error("Insert with empty ChatJID: want error, got nil")
	}
}

func TestUpsertChat_SetsNameWithoutTouchingLastTSOrUnread(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	mustInsert(t, s, ctx, msg("w1", "c1", 1000), true)
	if err := s.UpsertChat(ctx, "c1", "Alice"); err != nil {
		t.Fatalf("UpsertChat: %v", err)
	}

	name, ts, unread := chatRow(t, s, "c1")
	if name != "Alice" {
		t.Errorf("name = %q, want %q", name, "Alice")
	}
	if ts != 1000 {
		t.Errorf("last_ts = %d, want 1000 (UpsertChat must not touch it)", ts)
	}
	if unread != 1 {
		t.Errorf("unread = %d, want 1 (UpsertChat must not touch it)", unread)
	}
}

func TestUpsertChat_CreatesEmptyRow(t *testing.T) {
	s := openTempStore(t)

	if err := s.UpsertChat(context.Background(), "c1", "Alice"); err != nil {
		t.Fatalf("UpsertChat: %v", err)
	}

	name, ts, unread := chatRow(t, s, "c1")
	if name != "Alice" || ts != 0 || unread != 0 {
		t.Errorf("got (%q, %d, %d), want (%q, 0, 0)", name, ts, unread, "Alice")
	}
}

func TestMarkRead_ResetsUnread(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	mustInsert(t, s, ctx, msg("w1", "c1", 1000), true)
	mustInsert(t, s, ctx, msg("w2", "c1", 1001), true)
	if _, _, unread := chatRow(t, s, "c1"); unread != 2 {
		t.Fatalf("setup: unread = %d, want 2", unread)
	}

	if err := s.MarkRead(ctx, "c1"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if _, _, unread := chatRow(t, s, "c1"); unread != 0 {
		t.Errorf("unread after MarkRead = %d, want 0", unread)
	}
}

func mustInsert(t *testing.T, s *Store, ctx context.Context, m Message, bump bool) {
	t.Helper()
	if _, err := s.Insert(ctx, m, bump); err != nil {
		t.Fatalf("Insert(%s): %v", m.WAID, err)
	}
}
