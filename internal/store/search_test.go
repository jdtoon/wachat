package store

import (
	"context"
	"strings"
	"testing"
)

// TestFTSAvailable_OnFreshDB is a smoke test that the SQLite driver we
// link includes FTS5 support. If it ever stops compiling FTS5 (a build
// of modernc.org/sqlite without the right tag, say) this fails loud
// and early instead of silently breaking search.
func TestFTSAvailable_OnFreshDB(t *testing.T) {
	s := openTempStore(t)
	var name string
	if err := s.DB().QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='messages_fts'",
	).Scan(&name); err != nil {
		t.Fatalf("messages_fts missing: %v (FTS5 unavailable?)", err)
	}
}

func TestSearch_InsertIsIndexedByTrigger(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, context.Background(), Message{
		WAID: "w1", ChatJID: "c1", TS: 1000, Body: "the quick brown fox",
	}, false)
	mustInsert(t, s, context.Background(), Message{
		WAID: "w2", ChatJID: "c1", TS: 1001, Body: "lazy dog",
	}, false)

	hits, err := s.Search(ctx, "quick", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	if hits[0].WAID != "w1" {
		t.Errorf("hit.WAID = %q, want w1", hits[0].WAID)
	}
	if !strings.Contains(hits[0].Snippet, "[[quick]]") {
		t.Errorf("snippet missing highlight: %q", hits[0].Snippet)
	}
}

func TestSearch_DeleteRemovesFromIndex(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, context.Background(), Message{
		WAID: "w1", ChatJID: "c1", TS: 1000, Body: "unique-term-zzz",
	}, false)
	if _, err := s.DB().ExecContext(ctx, "DELETE FROM messages WHERE wa_id=?", "w1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	hits, err := s.Search(ctx, "unique-term-zzz", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits after delete, got %d", len(hits))
	}
}

func TestSearch_UpdateReindexesByTrigger(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, context.Background(), Message{
		WAID: "w1", ChatJID: "c1", TS: 1000, Body: "original term",
	}, false)
	if _, err := s.DB().ExecContext(ctx,
		"UPDATE messages SET body=? WHERE wa_id=?", "edited term", "w1",
	); err != nil {
		t.Fatalf("update: %v", err)
	}
	hits, _ := s.Search(ctx, "original", 10)
	if len(hits) != 0 {
		t.Errorf("'original' still matches after update; trigger broken")
	}
	hits, _ = s.Search(ctx, "edited", 10)
	if len(hits) != 1 {
		t.Errorf("'edited' should match; got %d hits", len(hits))
	}
}

func TestSearch_EmptyQueryReturnsNothing(t *testing.T) {
	s := openTempStore(t)
	hits, err := s.Search(context.Background(), "   ", 10)
	if err != nil {
		t.Errorf("Search with whitespace query errored: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("empty query returned %d hits, want 0", len(hits))
	}
}

func TestSearch_LimitHonored(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	for i := 0; i < 25; i++ {
		mustInsert(t, s, context.Background(), Message{
			WAID: msgID(i), ChatJID: "c1", TS: int64(1000 + i),
			Body: "common term in many rows",
		}, false)
	}
	hits, err := s.Search(ctx, "common", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 5 {
		t.Errorf("limit honored = %d, want 5", len(hits))
	}
}

func TestSearch_DiacriticsAreFolded(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, context.Background(), Message{
		WAID: "w1", ChatJID: "c1", TS: 1000, Body: "café au lait",
	}, false)
	// Search without the accent should still find it (tokenizer fold).
	hits, err := s.Search(ctx, "cafe", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("diacritic-folded search got %d hits, want 1", len(hits))
	}
}

func TestSearch_QuotesAreSafe(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, context.Background(), Message{
		WAID: "w1", ChatJID: "c1", TS: 1000, Body: `she said "hello"`,
	}, false)
	// A query containing a double quote should not blow up FTS5's parser.
	if _, err := s.Search(ctx, `she said "hello"`, 10); err != nil {
		t.Errorf("query with double quotes errored: %v", err)
	}
}

func TestSearch_JoinsChatName(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	if err := s.UpsertChat(ctx, "c1", "Alice"); err != nil {
		t.Fatalf("UpsertChat: %v", err)
	}
	mustInsert(t, s, context.Background(), Message{
		WAID: "w1", ChatJID: "c1", TS: 1000, Body: "weather is awful today",
	}, false)
	hits, _ := s.Search(ctx, "weather", 10)
	if len(hits) != 1 || hits[0].ChatName != "Alice" {
		t.Errorf("expected ChatName='Alice', got %+v", hits)
	}
}

func TestFTSPhrase_WrapsAndDeDoubleQuotes(t *testing.T) {
	if got := ftsPhrase(`hello`); got != `"hello"` {
		t.Errorf("ftsPhrase(hello) = %q, want %q", got, `"hello"`)
	}
	if got := ftsPhrase(`he said "hi"`); got != `"he said 'hi'"` {
		t.Errorf("ftsPhrase double-quote: got %q", got)
	}
}

// msgID makes a stable wa_id for the search tests.
func msgID(i int) string { return fmtPad("w", i) }

// fmtPad is defined in paging_test.go-adjacent helpers — replicate the
// minimal version here so this file isn't ordering-sensitive.
func fmtPad(prefix string, i int) string {
	const pad = "00000"
	s := itoa(i)
	if len(s) < len(pad) {
		s = pad[len(s):] + s
	}
	return prefix + s
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
