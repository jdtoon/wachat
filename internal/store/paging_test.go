package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// seed inserts n messages into chatJID with ts=1..n. Returns the wa_ids in
// insertion order.
func seed(t *testing.T, s *Store, chatJID string, n int) []string {
	t.Helper()
	ctx := context.Background()
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		w := fmt.Sprintf("w%05d", i+1)
		ids[i] = w
		if _, err := s.Insert(ctx, Message{
			WAID:    w,
			ChatJID: chatJID,
			TS:      int64(i + 1),
			Body:    w,
		}, true); err != nil {
			t.Fatalf("seed %s: %v", w, err)
		}
	}
	return ids
}

func TestPageOlder_NewestFirst(t *testing.T) {
	s := openTempStore(t)
	seed(t, s, "c1", 5)

	page, _, err := s.PageOlder(context.Background(), "c1", Cursor{}, 10)
	if err != nil {
		t.Fatalf("PageOlder: %v", err)
	}
	if len(page) != 5 {
		t.Fatalf("got %d messages, want 5", len(page))
	}
	for i := 0; i < len(page)-1; i++ {
		if page[i].TS < page[i+1].TS {
			t.Errorf("page[%d].TS=%d < page[%d].TS=%d (must be DESC)",
				i, page[i].TS, i+1, page[i+1].TS)
		}
	}
	if page[0].TS != 5 {
		t.Errorf("newest TS = %d, want 5", page[0].TS)
	}
}

func TestPageOlder_KeysetCoversEverything(t *testing.T) {
	const total = 500
	const pageSize = 50

	s := openTempStore(t)
	seed(t, s, "c1", total)

	seen := make(map[int64]bool, total)
	cursor := Cursor{}
	pages := 0
	for {
		page, next, err := s.PageOlder(context.Background(), "c1", cursor, pageSize)
		if err != nil {
			t.Fatalf("PageOlder: %v", err)
		}
		pages++
		if pages > total/pageSize+5 {
			t.Fatalf("too many pages (%d) — cursor not advancing?", pages)
		}
		for _, m := range page {
			if seen[m.TS] {
				t.Errorf("duplicate TS %d across pages", m.TS)
			}
			seen[m.TS] = true
		}
		if len(page) < pageSize {
			break
		}
		cursor = next
	}
	if len(seen) != total {
		t.Errorf("saw %d unique messages, want %d", len(seen), total)
	}
}

func TestPageOlder_HandlesTiedTimestamps(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	// 10 messages all at ts=1000. The (ts, id) cursor must page through
	// them without skipping or duplicating any.
	for i := 0; i < 10; i++ {
		if _, err := s.Insert(ctx, Message{
			WAID:    fmt.Sprintf("w%d", i),
			ChatJID: "c1",
			TS:      1000,
			Body:    fmt.Sprintf("m%d", i),
		}, false); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	seen := make(map[string]bool, 10)
	cursor := Cursor{}
	for {
		page, next, err := s.PageOlder(ctx, "c1", cursor, 3)
		if err != nil {
			t.Fatalf("PageOlder: %v", err)
		}
		for _, m := range page {
			if seen[m.WAID] {
				t.Errorf("duplicate WAID %q across pages with tied ts", m.WAID)
			}
			seen[m.WAID] = true
		}
		if len(page) < 3 {
			break
		}
		cursor = next
	}
	if len(seen) != 10 {
		t.Errorf("paged through %d tied-ts messages, want 10", len(seen))
	}
}

func TestPageOlder_EmptyChat(t *testing.T) {
	s := openTempStore(t)
	page, next, err := s.PageOlder(context.Background(), "nobody", Cursor{}, 10)
	if err != nil {
		t.Fatalf("PageOlder: %v", err)
	}
	if len(page) != 0 {
		t.Errorf("len(page) = %d, want 0", len(page))
	}
	if !next.IsZero() {
		t.Errorf("next cursor = %+v, want zero", next)
	}
}

func TestPageOlder_LimitHonored(t *testing.T) {
	s := openTempStore(t)
	seed(t, s, "c1", 20)

	page, _, err := s.PageOlder(context.Background(), "c1", Cursor{}, 7)
	if err != nil {
		t.Fatalf("PageOlder: %v", err)
	}
	if len(page) != 7 {
		t.Errorf("len(page) = %d, want 7", len(page))
	}
}

func TestPageOlder_ValidatesArgs(t *testing.T) {
	s := openTempStore(t)
	if _, _, err := s.PageOlder(context.Background(), "", Cursor{}, 10); err == nil {
		t.Error("empty chatJID: want error")
	}
	if _, _, err := s.PageOlder(context.Background(), "c1", Cursor{}, 0); err == nil {
		t.Error("limit=0: want error")
	}
	if _, _, err := s.PageOlder(context.Background(), "c1", Cursor{}, -5); err == nil {
		t.Error("limit<0: want error")
	}
}

// BenchmarkPageOlder_Deep proves keyset paging stays O(page) regardless of
// how deep into history we go. We page from near the start of a 10k-msg
// history and compare against paging from the end (the first page).
func BenchmarkPageOlder_Deep(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping deep-history benchmark in -short mode")
	}
	s, cleanup := openBenchStore(b)
	defer cleanup()

	const total = 10_000
	const pageSize = 50
	for i := 0; i < total; i++ {
		if _, err := s.Insert(context.Background(), Message{
			WAID:    fmt.Sprintf("w%05d", i),
			ChatJID: "c1",
			TS:      int64(i + 1),
		}, false); err != nil {
			b.Fatalf("seed: %v", err)
		}
	}

	// Walk to a deep cursor (near the start of history).
	cursor := Cursor{}
	for i := 0; i < (total-pageSize)/pageSize; i++ {
		_, next, err := s.PageOlder(context.Background(), "c1", cursor, pageSize)
		if err != nil {
			b.Fatalf("walk: %v", err)
		}
		cursor = next
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := s.PageOlder(context.Background(), "c1", cursor, pageSize)
		if err != nil {
			b.Fatalf("PageOlder: %v", err)
		}
	}
}

// TestPageOlder_DeepLatencyIsBounded is a non-bench assertion that a deep
// page is not dramatically slower than the first page (a regression guard
// for the keyset property). Loose factor so this is not flaky.
func TestPageOlder_DeepLatencyIsBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency assertion in -short mode")
	}
	s := openTempStore(t)
	ctx := context.Background()

	const total = 5_000
	const pageSize = 50
	for i := 0; i < total; i++ {
		if _, err := s.Insert(ctx, Message{
			WAID:    fmt.Sprintf("w%05d", i),
			ChatJID: "c1",
			TS:      int64(i + 1),
		}, false); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	timeOne := func(c Cursor) time.Duration {
		// Warm up the prepared statement / page cache.
		_, _, _ = s.PageOlder(ctx, "c1", c, pageSize)
		start := time.Now()
		for i := 0; i < 100; i++ {
			_, _, err := s.PageOlder(ctx, "c1", c, pageSize)
			if err != nil {
				t.Fatalf("PageOlder: %v", err)
			}
		}
		return time.Since(start) / 100
	}

	// First page = newest. Walk to a deep cursor (90% of the way in).
	deep := Cursor{}
	for i := 0; i < (total*9/10)/pageSize; i++ {
		_, n, err := s.PageOlder(ctx, "c1", deep, pageSize)
		if err != nil {
			t.Fatalf("walk: %v", err)
		}
		deep = n
	}

	first := timeOne(Cursor{})
	deepDur := timeOne(deep)

	// 25x is a very loose bound — in practice the ratio is ~1x on a warm
	// cache. We only want this to fail if someone reintroduces OFFSET.
	if deepDur > 25*first && deepDur > 5*time.Millisecond {
		t.Errorf("deep page took %v vs first page %v (ratio %.1fx) — keyset property may be broken",
			deepDur, first, float64(deepDur)/float64(first))
	}
}

// openBenchStore mirrors openTempStore but takes a testing.B.
func openBenchStore(b *testing.B) (*Store, func()) {
	b.Helper()
	dir := b.TempDir()
	s, err := Open(context.Background(), dir+"/wachat.db")
	if err != nil {
		b.Fatalf("Open: %v", err)
	}
	return s, func() { _ = s.Close() }
}
