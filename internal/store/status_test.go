package store

import (
	"context"
	"testing"
)

func TestUpdateStatus_RoundTrip(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()

	mustInsert(t, s, ctx, Message{
		WAID: "w1", ChatJID: "c1", TS: 1, Body: "hi",
	}, false)
	if err := s.UpdateStatus(ctx, "w1", StatusDelivered); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	var status string
	if err := s.DB().QueryRow("SELECT status FROM messages WHERE wa_id=?", "w1").Scan(&status); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != StatusDelivered {
		t.Errorf("status = %q, want %q", status, StatusDelivered)
	}
}

func TestUpdateStatus_UnknownWAIDIsNoOp(t *testing.T) {
	s := openTempStore(t)
	if err := s.UpdateStatus(context.Background(), "never-existed", StatusDelivered); err != nil {
		t.Errorf("unknown WAID should be a no-op, got %v", err)
	}
}

func TestUpdateStatus_RequiresArgs(t *testing.T) {
	s := openTempStore(t)
	if err := s.UpdateStatus(context.Background(), "", StatusDelivered); err == nil {
		t.Error("empty waID: want error")
	}
	if err := s.UpdateStatus(context.Background(), "w1", ""); err == nil {
		t.Error("empty status: want error")
	}
}

func TestInsert_DefaultStatusIsSent(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, ctx, Message{WAID: "w1", ChatJID: "c1", TS: 1, Body: "hi"}, false)

	var status string
	if err := s.DB().QueryRow("SELECT status FROM messages WHERE wa_id=?", "w1").Scan(&status); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != StatusSent {
		t.Errorf("default status = %q, want %q", status, StatusSent)
	}
}

func TestInsert_StatusFieldHonored(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, ctx, Message{
		WAID: "w1", ChatJID: "c1", TS: 1, Body: "hi", Status: StatusPending,
	}, false)

	var status string
	if err := s.DB().QueryRow("SELECT status FROM messages WHERE wa_id=?", "w1").Scan(&status); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != StatusPending {
		t.Errorf("status = %q, want %q", status, StatusPending)
	}
}

func TestSetStarred_RoundTrip(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, ctx, Message{WAID: "w1", ChatJID: "c1", TS: 1, Body: "hi"}, false)

	if err := s.SetStarred(ctx, "w1", true); err != nil {
		t.Fatalf("SetStarred: %v", err)
	}
	page, _, _ := s.PageOlder(ctx, "c1", Cursor{}, 10)
	if len(page) != 1 || !page[0].Starred {
		t.Errorf("Starred not round-tripped: %+v", page)
	}

	if err := s.SetStarred(ctx, "w1", false); err != nil {
		t.Fatalf("SetStarred unstar: %v", err)
	}
	page, _, _ = s.PageOlder(ctx, "c1", Cursor{}, 10)
	if page[0].Starred {
		t.Errorf("unstar didn't clear flag")
	}
}

func TestSetStarred_UnknownWAIDIsNoOp(t *testing.T) {
	s := openTempStore(t)
	if err := s.SetStarred(context.Background(), "nope", true); err != nil {
		t.Errorf("unknown waID should be no-op, got %v", err)
	}
}

func TestSetStarred_RequiresWAID(t *testing.T) {
	s := openTempStore(t)
	if err := s.SetStarred(context.Background(), "", true); err == nil {
		t.Error("empty waID: want error")
	}
}

func TestListStarred_FiltersAndOrders(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	for i, body := range []string{"first", "second", "third"} {
		mustInsert(t, s, ctx, Message{
			WAID: fmtPad("w", i), ChatJID: "c1", TS: int64(i + 1), Body: body,
		}, false)
	}
	// Star only w0 and w2.
	_ = s.SetStarred(ctx, "w00000", true)
	_ = s.SetStarred(ctx, "w00002", true)

	hits, err := s.ListStarred(ctx, 10)
	if err != nil {
		t.Fatalf("ListStarred: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2", len(hits))
	}
	// Newest first.
	if hits[0].Body != "third" || hits[1].Body != "first" {
		t.Errorf("ListStarred order: %+v", hits)
	}
}

func TestApplyEdit_UpdatesBodyAndFlag(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, ctx, Message{WAID: "w1", ChatJID: "c1", TS: 1, Body: "original"}, false)

	if err := s.ApplyEdit(ctx, "w1", "edited!"); err != nil {
		t.Fatalf("ApplyEdit: %v", err)
	}

	page, _, _ := s.PageOlder(ctx, "c1", Cursor{}, 10)
	if len(page) != 1 {
		t.Fatalf("want 1 message, got %d", len(page))
	}
	if page[0].Body != "edited!" {
		t.Errorf("body = %q, want edited!", page[0].Body)
	}
	if !page[0].Edited {
		t.Errorf("Edited = false, want true")
	}
}

func TestApplyEdit_UnknownWAIDIsNoOp(t *testing.T) {
	s := openTempStore(t)
	if err := s.ApplyEdit(context.Background(), "nope", "x"); err != nil {
		t.Errorf("unknown waID should be no-op, got %v", err)
	}
}

func TestApplyRevoke_FlipsFlag(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, ctx, Message{WAID: "w1", ChatJID: "c1", TS: 1, Body: "private"}, false)

	if err := s.ApplyRevoke(ctx, "w1"); err != nil {
		t.Fatalf("ApplyRevoke: %v", err)
	}

	page, _, _ := s.PageOlder(ctx, "c1", Cursor{}, 10)
	if !page[0].Revoked {
		t.Errorf("Revoked = false, want true")
	}
}

func TestInsert_QuoteFieldsRoundTrip(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, ctx, Message{
		WAID: "w2", ChatJID: "c1", TS: 1, Body: "reply",
		QuotedWAID: "w1", QuotedBody: "original text", QuotedSender: "alice",
	}, false)

	page, _, _ := s.PageOlder(ctx, "c1", Cursor{}, 10)
	m := page[0]
	if m.QuotedWAID != "w1" || m.QuotedBody != "original text" || m.QuotedSender != "alice" {
		t.Errorf("quote fields not round-tripped: %+v", m)
	}
}

func TestPageOlder_PopulatesStatus(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	mustInsert(t, s, ctx, Message{
		WAID: "w1", ChatJID: "c1", TS: 1, Body: "hi", Status: StatusRead,
	}, false)

	page, _, err := s.PageOlder(ctx, "c1", Cursor{}, 10)
	if err != nil {
		t.Fatalf("PageOlder: %v", err)
	}
	if len(page) != 1 || page[0].Status != StatusRead {
		t.Errorf("PageOlder didn't return Status=Read, got %+v", page)
	}
}
