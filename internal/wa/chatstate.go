package wa

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// ChatStateStore is the subset of *store.Store the pin/mute/archive
// handlers need. Defined narrowly so fake stores can stub it.
type ChatStateStore interface {
	SetPinned(ctx context.Context, jid string, pinned bool) error
	SetArchived(ctx context.Context, jid string, archived bool) error
	SetMuteUntil(ctx context.Context, jid string, muteUntilMS int64) error
}

func (h *Handler) applyPin(ctx context.Context, e *events.Pin) {
	store, ok := h.Store.(ChatStateStore)
	if !ok || e == nil || e.Action == nil {
		return
	}
	if err := store.SetPinned(ctx, e.JID.String(), e.Action.GetPinned()); err != nil && h.Logger != nil {
		h.Logger(fmt.Errorf("wa.applyPin %s: %w", e.JID, err))
	}
	if h.Notify != nil {
		h.Notify()
	}
}

func (h *Handler) applyArchive(ctx context.Context, e *events.Archive) {
	store, ok := h.Store.(ChatStateStore)
	if !ok || e == nil || e.Action == nil {
		return
	}
	if err := store.SetArchived(ctx, e.JID.String(), e.Action.GetArchived()); err != nil && h.Logger != nil {
		h.Logger(fmt.Errorf("wa.applyArchive %s: %w", e.JID, err))
	}
	if h.Notify != nil {
		h.Notify()
	}
}

func (h *Handler) applyMute(ctx context.Context, e *events.Mute) {
	store, ok := h.Store.(ChatStateStore)
	if !ok || e == nil || e.Action == nil {
		return
	}
	var until int64
	if e.Action.GetMuted() {
		// MuteAction.MuteEndTimestamp is unix seconds. -1 = forever.
		ts := e.Action.GetMuteEndTimestamp()
		switch {
		case ts < 0:
			until = -1
		case ts == 0:
			until = -1 // muted with no explicit end = forever in our convention
		default:
			until = ts * 1000
		}
	}
	if err := store.SetMuteUntil(ctx, e.JID.String(), until); err != nil && h.Logger != nil {
		h.Logger(fmt.Errorf("wa.applyMute %s: %w", e.JID, err))
	}
	if h.Notify != nil {
		h.Notify()
	}
}

// publishChatPresence dispatches a typing notification via the
// optional OnTyping callback. Stays in-memory (no DB write) — typing
// indicators are inherently transient.
func (h *Handler) publishChatPresence(e *events.ChatPresence) {
	if h == nil || h.OnTyping == nil || e == nil {
		return
	}
	composing := e.State == types.ChatPresenceComposing
	h.OnTyping(e.MessageSource.Chat.String(), e.MessageSource.Sender.String(), composing)
	if h.Notify != nil {
		h.Notify()
	}
}
