package store

import (
	"context"
	"testing"
)

func TestInsertBatch_EmptyIsNoOp(t *testing.T) {
	s := openTempStore(t)
	n, err := s.InsertBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("nil batch: %v", err)
	}
	if n != 0 {
		t.Errorf("nil batch created=%d, want 0", n)
	}
}

func TestInsertBatch_AllRowsCreated(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	msgs := []Message{
		{WAID: "a", ChatJID: "c1", TS: 100, Body: "hi"},
		{WAID: "b", ChatJID: "c1", TS: 200, Body: "yo"},
		{WAID: "c", ChatJID: "c2", TS: 300, Body: "hey"},
	}
	n, err := s.InsertBatch(ctx, msgs)
	if err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	if n != 3 {
		t.Errorf("created=%d, want 3", n)
	}

	// Both chats should exist with the right last_ts.
	row := func(jid string) (lastTS int64, unread int) {
		t.Helper()
		if err := s.DB().QueryRow(
			"SELECT COALESCE(last_ts,0), unread FROM chats WHERE jid=?", jid,
		).Scan(&lastTS, &unread); err != nil {
			t.Fatalf("scan %s: %v", jid, err)
		}
		return
	}
	if ts, _ := row("c1"); ts != 200 {
		t.Errorf("c1 last_ts = %d, want 200", ts)
	}
	if ts, _ := row("c2"); ts != 300 {
		t.Errorf("c2 last_ts = %d, want 300", ts)
	}
}

func TestInsertBatch_DedupsOnWAID(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	mustInsert(t, s, ctx, Message{WAID: "a", ChatJID: "c1", TS: 100, Body: "first"}, false)

	// Batch with one new + one duplicate.
	n, err := s.InsertBatch(ctx, []Message{
		{WAID: "a", ChatJID: "c1", TS: 100, Body: "redelivery"},
		{WAID: "b", ChatJID: "c1", TS: 200, Body: "new"},
	})
	if err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	if n != 1 {
		t.Errorf("created=%d, want 1 (one dedup, one new)", n)
	}

	var total int
	if err := s.DB().QueryRow("SELECT COUNT(*) FROM messages").Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 2 {
		t.Errorf("total rows = %d, want 2 (no duplicate)", total)
	}
}

func TestInsertBatch_SkipsRowsMissingRequiredFields(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	msgs := []Message{
		{WAID: "", ChatJID: "c1", TS: 100, Body: "no wa_id"},
		{WAID: "b", ChatJID: "", TS: 100, Body: "no chat_jid"},
		{WAID: "c", ChatJID: "c1", TS: 200, Body: "ok"},
	}
	n, err := s.InsertBatch(ctx, msgs)
	if err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	if n != 1 {
		t.Errorf("created=%d, want 1 (only the valid row)", n)
	}
}

func TestInsertBatch_DoesNotBumpUnread(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	msgs := []Message{
		{WAID: "a", ChatJID: "c1", TS: 100, Body: "hi"},
		{WAID: "b", ChatJID: "c1", TS: 200, Body: "yo"},
	}
	if _, err := s.InsertBatch(ctx, msgs); err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}

	var unread int
	if err := s.DB().QueryRow(
		"SELECT unread FROM chats WHERE jid=?", "c1",
	).Scan(&unread); err != nil {
		t.Fatalf("scan unread: %v", err)
	}
	if unread != 0 {
		t.Errorf("unread after batch = %d, want 0 (history sync must not bump)", unread)
	}
}

func TestInsertBatch_FeedsFTSIndex(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	if _, err := s.InsertBatch(ctx, []Message{
		{WAID: "a", ChatJID: "c1", TS: 100, Body: "history term shibboleth"},
	}); err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	hits, err := s.Search(ctx, "shibboleth", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("FTS5 should index batch-inserted messages; got %d hits", len(hits))
	}
}

func TestInsertBatch_PerfRoughlyLinear(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in -short")
	}
	s := openTempStore(t)
	ctx := context.Background()

	const N = 2000
	msgs := make([]Message, N)
	for i := range msgs {
		msgs[i] = Message{
			WAID:    fmtPad("w", i),
			ChatJID: "c1",
			TS:      int64(i + 1),
			Body:    "lorem ipsum dolor sit amet",
		}
	}
	if _, err := s.InsertBatch(ctx, msgs); err != nil {
		t.Fatalf("InsertBatch %d: %v", N, err)
	}
	// Smoke-only: just verify all N landed; the perf comparison vs.
	// per-row Insert lives in the bench harness.
	var total int
	if err := s.DB().QueryRow("SELECT COUNT(*) FROM messages").Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != N {
		t.Errorf("after batch of %d: total=%d", N, total)
	}
}
