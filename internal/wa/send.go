package wa

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// SendText sends a plain-text message to chatJID and returns the
// server-assigned message ID (whatsmeow's MessageID = our wa_id) plus
// the server timestamp.
//
// Signature verified against pkg.go.dev/go.mau.fi/whatsmeow on write
// (SendMessage now takes a context and returns SendResponse — both
// changed since older examples).
//
// SendText does NOT persist the message to the store. The caller is
// expected to optimistically insert a "pending" bubble in the UI via
// state.AddOptimistic and let the dedup path in store.Insert reconcile
// when the matching incoming event arrives. This keeps the send path
// fast (no DB write before the network round-trip).
func (c *Client) SendText(ctx context.Context, chatJID, body string) (waID string, err error) {
	if c == nil || c.wm == nil {
		return "", fmt.Errorf("wa.SendText: client is nil")
	}
	if chatJID == "" {
		return "", fmt.Errorf("wa.SendText: chatJID is required")
	}
	if body == "" {
		return "", fmt.Errorf("wa.SendText: body is required")
	}
	to, err := types.ParseJID(chatJID)
	if err != nil {
		return "", fmt.Errorf("wa.SendText: parse JID %q: %w", chatJID, err)
	}
	resp, err := c.wm.SendMessage(ctx, to, &waE2E.Message{
		Conversation: proto.String(body),
	})
	if err != nil {
		return "", fmt.Errorf("wa.SendText: %w", err)
	}
	return resp.ID, nil
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
