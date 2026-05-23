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
	if _, err := c.SendText(context.Background(), "a@s.whatsapp.net", "hi"); err == nil {
		t.Error("expected error from nil-client SendText")
	}
}

func TestSendText_EmptyChatJIDErrors(t *testing.T) {
	c := newTempClient(t)
	if _, err := c.SendText(context.Background(), "", "hi"); err == nil || !strings.Contains(err.Error(), "chatJID") {
		t.Errorf("want chatJID error, got %v", err)
	}
}

func TestSendText_EmptyBodyErrors(t *testing.T) {
	c := newTempClient(t)
	if _, err := c.SendText(context.Background(), "a@s.whatsapp.net", ""); err == nil || !strings.Contains(err.Error(), "body") {
		t.Errorf("want body error, got %v", err)
	}
}

func TestSendText_InvalidJIDErrors(t *testing.T) {
	c := newTempClient(t)
	// Missing @server — ParseJID rejects.
	_, err := c.SendText(context.Background(), "no-at-sign", "hi")
	if err == nil {
		t.Error("expected parse error for malformed JID")
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
