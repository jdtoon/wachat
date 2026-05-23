package store

import (
	"context"
	"testing"
)

func seedAround(t *testing.T, s *Store) []int64 {
	t.Helper()
	ctx := context.Background()
	ids := make([]int64, 20)
	for i := 0; i < 20; i++ {
		m := Message{
			WAID:    fmtPad("w", i),
			ChatJID: "c1",
			TS:      int64(1000 + i),
			Body:    "msg",
		}
		_, err := s.Insert(ctx, m, false)
		if err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	rows, err := s.DB().QueryContext(ctx, `SELECT id FROM messages WHERE chat_jid='c1' ORDER BY ts ASC`)
	if err != nil {
		t.Fatalf("scan ids: %v", err)
	}
	defer rows.Close()
	out := ids[:0]
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan id: %v", err)
		}
		out = append(out, id)
	}
	return out
}

func TestPageAround_AnchorPresentInResult(t *testing.T) {
	s := openTempStore(t)
	ids := seedAround(t, s)
	anchor := ids[10]

	page, _, err := s.PageAround(context.Background(), "c1", anchor, 3, 3)
	if err != nil {
		t.Fatalf("PageAround: %v", err)
	}

	found := false
	for _, m := range page {
		if m.ID == anchor {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("anchor %d missing from PageAround result", anchor)
	}
}

func TestPageAround_RespectsBeforeAndAfter(t *testing.T) {
	s := openTempStore(t)
	ids := seedAround(t, s)
	anchor := ids[10]

	page, _, err := s.PageAround(context.Background(), "c1", anchor, 4, 2)
	if err != nil {
		t.Fatalf("PageAround: %v", err)
	}
	// 2 newer + 1 anchor + 4 older = 7 (assuming enough rows on both
	// sides, which there are with 20 seeded).
	if got := len(page); got != 7 {
		t.Errorf("page len = %d, want 7", got)
	}
}

func TestPageAround_NewestFirstOrder(t *testing.T) {
	s := openTempStore(t)
	ids := seedAround(t, s)
	anchor := ids[10]

	page, _, err := s.PageAround(context.Background(), "c1", anchor, 5, 5)
	if err != nil {
		t.Fatalf("PageAround: %v", err)
	}
	for i := 0; i < len(page)-1; i++ {
		if page[i].TS < page[i+1].TS {
			t.Errorf("PageAround not newest-first: page[%d].TS=%d < page[%d].TS=%d",
				i, page[i].TS, i+1, page[i+1].TS)
		}
	}
}

func TestPageAround_NearEndOfHistory(t *testing.T) {
	s := openTempStore(t)
	ids := seedAround(t, s)
	anchor := ids[1] // second-oldest

	page, _, err := s.PageAround(context.Background(), "c1", anchor, 5, 5)
	if err != nil {
		t.Fatalf("PageAround: %v", err)
	}
	// Only 1 older message exists; the older half is short, anchor +
	// up to 5 newer = up to 7 total. Just verify the anchor is there
	// and order is sane.
	hasAnchor := false
	for _, m := range page {
		if m.ID == anchor {
			hasAnchor = true
		}
	}
	if !hasAnchor {
		t.Error("anchor near history edge not present")
	}
}

func TestPageAround_UnknownAnchorErrors(t *testing.T) {
	s := openTempStore(t)
	if _, _, err := s.PageAround(context.Background(), "c1", 9999, 5, 5); err == nil {
		t.Error("expected error for unknown anchor")
	}
}

func TestPageAround_ValidatesArgs(t *testing.T) {
	s := openTempStore(t)
	ids := seedAround(t, s)
	if _, _, err := s.PageAround(context.Background(), "", ids[0], 1, 1); err == nil {
		t.Error("empty chatJID: want error")
	}
	if _, _, err := s.PageAround(context.Background(), "c1", ids[0], -1, 1); err == nil {
		t.Error("negative before: want error")
	}
}
