package wa

import (
	"context"
	"path/filepath"
	"testing"

	"go.mau.fi/whatsmeow"
)

func newTempClient(t *testing.T) *Client {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.db")
	c, err := New(context.Background(), path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestNew_FreshDBSucceeds(t *testing.T) {
	c := newTempClient(t)
	if c == nil {
		t.Fatal("New returned nil client")
	}
}

func TestNeedsPairing_FreshDBReturnsTrue(t *testing.T) {
	c := newTempClient(t)
	if !c.NeedsPairing() {
		t.Error("NeedsPairing() = false on a fresh session DB, want true")
	}
}

func TestNeedsPairing_NilSafe(t *testing.T) {
	var c *Client
	if !c.NeedsPairing() {
		t.Error("(nil *Client).NeedsPairing() = false, want true (defensive)")
	}
}

func TestDisconnect_NeverConnectedIsSafe(t *testing.T) {
	c := newTempClient(t)
	// Should not panic, should not return error (Disconnect has no return).
	c.Disconnect()
}

func TestClose_NeverConnectedIsSafe(t *testing.T) {
	c := newTempClient(t)
	if err := c.Close(); err != nil {
		t.Errorf("Close on never-connected client: %v", err)
	}
}

func TestAddEventHandler_RegistersBeforeConnect(t *testing.T) {
	c := newTempClient(t)
	id := c.AddEventHandler(func(any) {})
	if id == 0 {
		t.Error("AddEventHandler returned id=0; whatsmeow uses 0 as sentinel for not-registered")
	}
}

func TestAdaptQRChannel_ForwardsItemsAndClosesOnInputClose(t *testing.T) {
	in := make(chan whatsmeow.QRChannelItem, 3)
	in <- whatsmeow.QRChannelItem{Event: "code", Code: "abc"}
	in <- whatsmeow.QRChannelItem{Event: "code", Code: "def"}
	in <- whatsmeow.QRChannelItem{Event: "success"}
	close(in)

	out := adaptQRChannel(in)

	var got []QRItem
	for item := range out {
		got = append(got, item)
	}
	want := []QRItem{
		{Event: "code", Code: "abc"},
		{Event: "code", Code: "def"},
		{Event: "success"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
