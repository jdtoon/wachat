package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// openTempStore returns a Store backed by a fresh SQLite file in t.TempDir().
// Cleanup is registered via t.Cleanup.
func openTempStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wachat.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func queryString(t *testing.T, db *sql.DB, q string) string {
	t.Helper()
	var v string
	if err := db.QueryRow(q).Scan(&v); err != nil {
		t.Fatalf("query %q: %v", q, err)
	}
	return v
}

func TestOpen_AppliesWALAndSynchronous(t *testing.T) {
	s := openTempStore(t)

	if got := queryString(t, s.DB(), "PRAGMA journal_mode"); got != "wal" {
		t.Errorf("journal_mode = %q, want %q", got, "wal")
	}
	// synchronous=NORMAL is 1; FULL is 2.
	var sync int
	if err := s.DB().QueryRow("PRAGMA synchronous").Scan(&sync); err != nil {
		t.Fatalf("PRAGMA synchronous: %v", err)
	}
	if sync != 1 {
		t.Errorf("synchronous = %d, want 1 (NORMAL)", sync)
	}
}

func TestOpen_CreatesMessagesAndChatsTables(t *testing.T) {
	s := openTempStore(t)

	for _, table := range []string{"messages", "chats"} {
		var name string
		err := s.DB().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
	}
}

func TestOpen_CreatesChatTsIndex(t *testing.T) {
	s := openTempStore(t)

	var name string
	err := s.DB().QueryRow(
		"SELECT name FROM sqlite_master WHERE type='index' AND name='idx_chat_ts'",
	).Scan(&name)
	if err != nil {
		t.Fatalf("idx_chat_ts missing: %v", err)
	}

	// The index must cover the keyset hot path. Query plan should NOT be
	// "SCAN messages"; it should hit idx_chat_ts.
	rows, err := s.DB().Query(
		"EXPLAIN QUERY PLAN SELECT * FROM messages WHERE chat_jid=? AND ts<? ORDER BY ts DESC LIMIT 50",
		"x@s.whatsapp.net", 1_000_000_000_000,
	)
	if err != nil {
		t.Fatalf("explain query plan: %v", err)
	}
	defer rows.Close()

	var planUsesIndex bool
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan plan row: %v", err)
		}
		if contains(detail, "idx_chat_ts") {
			planUsesIndex = true
		}
	}
	if !planUsesIndex {
		t.Errorf("keyset query did not use idx_chat_ts; plan must hit the index")
	}
}

// contains is strings.Contains without importing strings just for this.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestOpen_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wachat.db")

	for i := 0; i < 3; i++ {
		s, err := Open(context.Background(), path)
		if err != nil {
			t.Fatalf("Open #%d: %v", i, err)
		}
		// Each re-open must still see the schema in place.
		if got := queryString(t, s.DB(), "PRAGMA journal_mode"); got != "wal" {
			t.Errorf("Open #%d: journal_mode = %q, want wal", i, got)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("Close #%d: %v", i, err)
		}
	}
}

func TestClose_NilSafe(t *testing.T) {
	var s *Store
	if err := s.Close(); err != nil {
		t.Errorf("nil Store.Close should be a no-op, got %v", err)
	}
}
