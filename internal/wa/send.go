package wa

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// SendText sends a plain-text message to chatJID using msgID as the
// WhatsApp message ID — caller mints it via GenerateID(), inserts the
// optimistic bubble in the store under that same ID, and (because the
// IDs match) avoids the briefly-double-bubble glitch from earlier
// versions.
//
// Returns the server timestamp; nil error means the message was
// accepted by the server (status transitions from pending → sent
// from the caller's perspective).
//
// Signatures verified against pkg.go.dev/go.mau.fi/whatsmeow.
func (c *Client) SendText(ctx context.Context, chatJID, body, msgID string) error {
	if c == nil || c.wm == nil {
		return fmt.Errorf("wa.SendText: client is nil")
	}
	if chatJID == "" {
		return fmt.Errorf("wa.SendText: chatJID is required")
	}
	if body == "" {
		return fmt.Errorf("wa.SendText: body is required")
	}
	to, err := types.ParseJID(chatJID)
	if err != nil {
		return fmt.Errorf("wa.SendText: parse JID %q: %w", chatJID, err)
	}
	extra := whatsmeow.SendRequestExtra{}
	if msgID != "" {
		extra.ID = msgID
	}
	if _, err := c.wm.SendMessage(ctx, to, &waE2E.Message{
		Conversation: proto.String(body),
	}, extra); err != nil {
		return fmt.Errorf("wa.SendText: %w", err)
	}
	return nil
}

// GenerateID returns a whatsmeow-format MessageID. The caller uses
// this to mint an outgoing message's WAID up front so the optimistic
// bubble and the eventually-confirmed receipt share the same row.
//
// Returns "" if the client is nil — call sites can detect this and
// fall back to a local placeholder.
func (c *Client) GenerateID() string {
	if c == nil || c.wm == nil {
		return ""
	}
	return c.wm.GenerateMessageID()
}

// OwnJID returns the locally paired account's JID, or empty if the
// device is not yet paired. Used by the UI to decide message
// alignment.
func (c *Client) OwnJID() string {
	if c == nil || c.wm == nil || c.wm.Store == nil || c.wm.Store.ID == nil {
		return ""
	}
	return c.wm.Store.ID.String()
}
