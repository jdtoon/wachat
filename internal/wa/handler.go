package wa

import (
	"context"
	"fmt"

	"github.com/jdtoon/wachat/internal/store"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// MessageEvent is wachat's normalized form of an incoming WhatsApp message.
// It exists so the UI layer never has to import whatsmeow's types — the
// boundary stays in this package (CLAUDE.md §3 / §4).
type MessageEvent struct {
	WAID      string
	ChatJID   string
	SenderJID string
	TS        int64 // unix millis
	Body      string
	FromMe    bool

	// Reply info — set when ContextInfo.QuotedMessage was present.
	QuotedWAID   string
	QuotedBody   string
	QuotedSender string
}

// Persister is the subset of *store.Store the handler needs. Defined as
// an interface so tests can substitute a fake without spinning up SQLite.
type Persister interface {
	Insert(ctx context.Context, m store.Message, bumpUnread bool) (bool, error)
}

// Handler implements the persist → channel → Notify pipeline described in
// CLAUDE.md §4. Its methods are called on whatsmeow's background
// goroutines; they must stay cheap and never mutate UI state.
//
// Out is the channel the UI goroutine drains. The send is non-blocking —
// if the buffer is full the event payload is dropped (the UI will repaint
// from the store on its next frame). Notify is always called regardless,
// so a slow UI can never stall the protocol layer.
type Handler struct {
	Store  Persister
	Out    chan<- MessageEvent
	Notify func()
	Logger func(error) // called for errors the EventHandler adapter can't return
	// OnConnState is invoked whenever the wa-layer connection state
	// changes (Connected / Disconnected / LoggedOut). Optional. When
	// non-nil and Notify is also set, the UI is woken right after the
	// callback so the banner repaints promptly.
	OnConnState func(ConnectionState)
}

// OnMessage runs the persist → send → notify pipeline for a single event.
// Order is significant:
//
//  1. Insert first — so a message reaches the store even if the UI buffer
//     is full and the channel send is dropped.
//  2. Non-blocking send second — best-effort payload delivery to the UI.
//  3. Notify last — wakes the UI to read from the store.
//
// Returns the Insert error (if any). A full channel is not an error.
func (h *Handler) OnMessage(ctx context.Context, ev MessageEvent) error {
	if h == nil || h.Store == nil {
		return fmt.Errorf("wa.Handler: Store is nil")
	}

	// Step 1: persist. We bump the chat's unread counter only for incoming
	// messages — messages we sent should not mark themselves unread.
	_, err := h.Store.Insert(ctx, store.Message{
		WAID:         ev.WAID,
		ChatJID:      ev.ChatJID,
		SenderJID:    ev.SenderJID,
		TS:           ev.TS,
		Body:         ev.Body,
		QuotedWAID:   ev.QuotedWAID,
		QuotedBody:   ev.QuotedBody,
		QuotedSender: ev.QuotedSender,
	}, !ev.FromMe)
	if err != nil {
		return fmt.Errorf("wa.Handler.OnMessage: persist: %w", err)
	}

	// Step 2: non-blocking send. A slow UI must never stall whatsmeow.
	if h.Out != nil {
		select {
		case h.Out <- ev:
		default:
		}
	}

	// Step 3: notify (wake the Gio frame loop).
	if h.Notify != nil {
		h.Notify()
	}
	return nil
}

// ConnectionState is wachat's wa-layer view of the network state.
// Mirrors internal/ui/ConnState so the boundary stays inside the wa
// package; main.go translates one into the other when wiring the
// banner.
type ConnectionState int

const (
	ConnectionConnected ConnectionState = iota
	ConnectionConnecting
	ConnectionDisconnected
	ConnectionLoggedOut
)

// OnConnState is a callback fed by Adapter when the wa-layer
// connection state changes. Set this on Handler to hear about
// Connected/Disconnected/LoggedOut without subscribing to whatsmeow's
// events directly.
//
// Called from whatsmeow goroutines — keep cheap (CLAUDE.md §4); the
// banner UI lives on the UI thread and reacts via Notify.

// Adapter returns a whatsmeow.EventHandler that decodes the events
// this build understands into wachat-local types. Other event types
// are silently ignored — the broader event surface lands as wachat
// grows.
//
// ownJIDFn is called when an event needs the local device's JID
// (currently just history sync, where the proto's MessageKey.FromMe
// flag has to be turned back into a sender JID). Pass nil to defer
// to the empty-sender fallback.
func (h *Handler) Adapter(ctx context.Context, ownJIDFn func() string) whatsmeow.EventHandler {
	return func(evt any) {
		switch e := evt.(type) {
		case *events.Message:
			// Protocol messages (edits, revokes, etc.) come wrapped in
			// a regular events.Message — they don't have a separate
			// event type. Detect them up front so we don't try to
			// persist a "new message" for an edit.
			if pm := e.Message.GetProtocolMessage(); pm != nil {
				h.handleProtocol(ctx, pm)
				return
			}
			// Reactions also ride inside events.Message.
			if rm := e.Message.GetReactionMessage(); rm != nil {
				h.handleReaction(ctx, e, rm)
				return
			}
			if err := h.OnMessage(ctx, fromWMMessage(e)); err != nil && h.Logger != nil {
				h.Logger(err)
			}
		case *events.HistorySync:
			ownJID := ""
			if ownJIDFn != nil {
				ownJID = ownJIDFn()
			}
			if err := h.OnHistorySync(ctx, e.Data, ownJID); err != nil && h.Logger != nil {
				h.Logger(err)
			}
		case *events.PushName:
			h.applyPushName(ctx, e.JID.String(), e.NewPushName)
		case *events.Receipt:
			h.OnReceipt(ctx, e.MessageIDs, e.Type)
		case *events.Connected:
			h.publishState(ConnectionConnected)
		case *events.Disconnected:
			h.publishState(ConnectionDisconnected)
		case *events.LoggedOut:
			h.publishState(ConnectionLoggedOut)
		case *events.PairSuccess:
			h.publishState(ConnectionConnected)
		}
	}
}

// applyPushName records a learned push name on the chats table if the
// chat is a 1:1 (the JID matches an existing chat row). Group chats
// have explicit names from the server and we don't overwrite those
// from a participant push name.
func (h *Handler) applyPushName(ctx context.Context, jid, pushName string) {
	if h == nil || jid == "" || pushName == "" {
		return
	}
	persister, ok := h.Store.(HistoryPersister)
	if !ok {
		return
	}
	// Best-effort — errors get logged via h.Logger but don't bubble.
	if err := persister.UpsertChat(ctx, jid, pushName); err != nil && h.Logger != nil {
		h.Logger(fmt.Errorf("wa.applyPushName %s: %w", jid, err))
	}
	if h.Notify != nil {
		h.Notify()
	}
}

// OnConnState may be set on Handler to receive connection-state
// updates derived from the events.Connected / events.Disconnected /
// events.LoggedOut stream. Optional — if nil, state changes are
// dropped silently.
//
// Defined as a field on Handler so we don't break the existing zero-
// value usage; older callers that only listen for messages keep
// working unchanged.
func (h *Handler) publishState(s ConnectionState) {
	if h == nil || h.OnConnState == nil {
		return
	}
	h.OnConnState(s)
	if h.Notify != nil {
		h.Notify()
	}
}

// fromWMMessage maps the whatsmeow event onto wachat's normalized struct.
// Kept package-private so the conversion stays inside the wa boundary.
func fromWMMessage(e *events.Message) MessageEvent {
	qWAID, qBody, qSender := extractQuoted(e.Message)
	return MessageEvent{
		WAID:         e.Info.ID,
		ChatJID:      e.Info.Chat.String(),
		SenderJID:    e.Info.Sender.String(),
		TS:           e.Info.Timestamp.UnixMilli(),
		Body:         extractBody(e.Message),
		FromMe:       e.Info.IsFromMe,
		QuotedWAID:   qWAID,
		QuotedBody:   qBody,
		QuotedSender: qSender,
	}
}
