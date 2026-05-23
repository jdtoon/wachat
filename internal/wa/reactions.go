package wa

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// ReactionSetter is the subset of *store.Store the reaction handler
// needs.
type ReactionSetter interface {
	SetReaction(ctx context.Context, targetWAID, senderJID, emoji string, ts int64) error
}

// handleReaction applies a ReactionMessage event to the reactions
// table. The target message id is in ReactionMessage.Key.ID; the
// sender is the event's Info.Sender; the emoji is Text (empty = remove).
func (h *Handler) handleReaction(ctx context.Context, e *events.Message, rm *waE2E.ReactionMessage) {
	if h == nil || rm == nil {
		return
	}
	setter, ok := h.Store.(ReactionSetter)
	if !ok {
		return
	}
	targetKey := rm.GetKey()
	if targetKey == nil {
		return
	}
	targetWAID := targetKey.GetID()
	if targetWAID == "" {
		return
	}
	sender := e.Info.Sender.String()
	emoji := rm.GetText()
	ts := rm.GetSenderTimestampMS()
	if ts == 0 {
		ts = e.Info.Timestamp.UnixMilli()
	}
	if err := setter.SetReaction(ctx, targetWAID, sender, emoji, ts); err != nil && h.Logger != nil {
		h.Logger(fmt.Errorf("wa.handleReaction: %s by %s: %w", targetWAID, sender, err))
	}
	if h.Notify != nil {
		h.Notify()
	}
}

// SendReaction sends a reaction emoji to a target message. Pass an
// empty emoji to remove a previously-set reaction.
//
// chatJID is the conversation (group or 1:1) the target lives in.
// targetSenderJID is the WhatsApp sender of the target message — for
// our own outgoing messages this is our OwnJID, for received ones it
// is the original sender's JID.
//
// Returns the WAID of the reaction message itself, in case the caller
// wants to dedup against the inbound reflection of our own reaction.
func (c *Client) SendReaction(ctx context.Context, chatJID, targetSenderJID, targetWAID, emoji string) (string, error) {
	if c == nil || c.wm == nil {
		return "", fmt.Errorf("wa.SendReaction: client is nil")
	}
	if targetWAID == "" {
		return "", fmt.Errorf("wa.SendReaction: targetWAID required")
	}
	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return "", fmt.Errorf("wa.SendReaction: parse chat %q: %w", chatJID, err)
	}
	sender, err := types.ParseJID(targetSenderJID)
	if err != nil {
		return "", fmt.Errorf("wa.SendReaction: parse target sender %q: %w", targetSenderJID, err)
	}
	msgID := c.wm.GenerateMessageID()
	resp, err := c.wm.SendMessage(ctx, chat,
		c.wm.BuildReaction(chat, sender, targetWAID, emoji),
		whatsmeow.SendRequestExtra{ID: msgID},
	)
	if err != nil {
		return "", fmt.Errorf("wa.SendReaction: %w", err)
	}
	return resp.ID, nil
}
