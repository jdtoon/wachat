package wa

import (
	"context"
	"strings"
	"testing"
)

// SendText with a nil client / empty args must return validation
// errors rather than crashing. We can't exercise a real send without a
// live whatsmeow session — those code paths are verified manually.

func TestSendText_NilClientErrors(t *testing.T) {
	var c *Client
	if err := c.SendText(context.Background(), "a@s.whatsapp.net", "hi", "ID1"); err == nil {
		t.Error("expected error from nil-client SendText")
	}
}

func TestSendText_EmptyChatJIDErrors(t *testing.T) {
	c := newTempClient(t)
	if err := c.SendText(context.Background(), "", "hi", "ID1"); err == nil || !strings.Contains(err.Error(), "chatJID") {
		t.Errorf("want chatJID error, got %v", err)
	}
}

func TestSendText_EmptyBodyErrors(t *testing.T) {
	c := newTempClient(t)
	if err := c.SendText(context.Background(), "a@s.whatsapp.net", "", "ID1"); err == nil || !strings.Contains(err.Error(), "body") {
		t.Errorf("want body error, got %v", err)
	}
}

func TestSendText_InvalidJIDErrors(t *testing.T) {
	c := newTempClient(t)
	// Missing @server — ParseJID rejects.
	err := c.SendText(context.Background(), "no-at-sign", "hi", "ID1")
	if err == nil {
		t.Error("expected parse error for malformed JID")
	}
}

func TestGenerateID_PairedClientReturnsNonEmpty(t *testing.T) {
	c := newTempClient(t)
	id := c.GenerateID()
	if id == "" {
		t.Error("GenerateID on a fresh client returned empty (should still work pre-pair)")
	}
}

func TestGenerateID_NilClientReturnsEmpty(t *testing.T) {
	var c *Client
	if got := c.GenerateID(); got != "" {
		t.Errorf("nil GenerateID = %q, want empty", got)
	}
}

func TestOwnJID_UnpairedReturnsEmpty(t *testing.T) {
	c := newTempClient(t)
	if got := c.OwnJID(); got != "" {
		t.Errorf("OwnJID on unpaired device = %q, want empty", got)
	}
}

func TestOwnJID_NilClientIsEmpty(t *testing.T) {
	var c *Client
	if got := c.OwnJID(); got != "" {
		t.Errorf("(nil).OwnJID = %q, want empty", got)
	}
}
