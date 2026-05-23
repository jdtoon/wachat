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
