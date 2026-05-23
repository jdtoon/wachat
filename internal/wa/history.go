package wa

import (
	"context"
	"fmt"

	"github.com/jdtoon/wachat/internal/store"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
)

// HistoryResult is the wachat-normalized view of a single HistorySync
// event after extraction from the whatsmeow protobuf. Stays decoupled
// from the proto types so the persistence + tests don't drag them in.
type HistoryResult struct {
	// Chats lists every conversation the sync mentioned, with the
	// display name (Conversation.Name) if present.
	Chats []HistoryChat
	// Messages is the flat list of historical messages across all
	// conversations, ready to hand to store.InsertBatch.
	Messages []store.Message
	// PushNames maps user JID → push name learned during the sync.
	// Folded into chat names by OnHistorySync for 1:1 chats.
	PushNames map[string]string
}

// HistoryChat is one conversation summary from the sync.
type HistoryChat struct {
	JID  string
	Name string
}

// fromWMHistorySync converts a whatsmeow HistorySync proto into the
// wachat-local form. Pure function — no Gio, no store, no whatsmeow
// client. Tested directly.
//
// ownJID is the local device's JID so we can mark our own historical
// messages with the right SenderJID; pass empty if we don't know yet
// (the bubble alignment will fall back to "empty sender = from me",
// matching the runtime behavior in internal/ui).
func fromWMHistorySync(data *waHistorySync.HistorySync, ownJID string) HistoryResult {
	out := HistoryResult{PushNames: make(map[string]string)}
	if data == nil {
		return out
	}

	for _, pn := range data.GetPushnames() {
		id := pn.GetID()
		name := pn.GetPushname()
		if id != "" && name != "" {
			out.PushNames[id] = name
		}
	}

	for _, conv := range data.GetConversations() {
		jid := conv.GetID()
		if jid == "" {
			continue
		}
		out.Chats = append(out.Chats, HistoryChat{
			JID:  jid,
			Name: conv.GetName(),
		})

		for _, hm := range conv.GetMessages() {
			wmi := hm.GetMessage()
			if wmi == nil {
				continue
			}
			key := wmi.GetKey()
			if key == nil {
				continue
			}
			waID := key.GetID()
			if waID == "" {
				continue
			}

			sender := senderFromKey(key, jid, ownJID)
			body := extractBody(wmi.GetMessage())

			out.Messages = append(out.Messages, store.Message{
				WAID:      waID,
				ChatJID:   jid,
				SenderJID: sender,
				TS:        int64(wmi.GetMessageTimestamp()) * 1000,
				Body:      body,
			})
		}
	}
	return out
}

// senderFromKey computes the wachat-local SenderJID convention from a
// WhatsApp MessageKey. FromMe messages use ownJID (so bubble alignment
// matches the live flow); group messages use the Participant field;
// 1:1 received messages use the chat's JID (the other party).
func senderFromKey(key keyAccessor, chatJID, ownJID string) string {
	if key.GetFromMe() {
		return ownJID
	}
	if part := key.GetParticipant(); part != "" {
		return part
	}
	return chatJID
}

// keyAccessor is the subset of MessageKey wa.history.go needs. Defined
// as an interface so senderFromKey can be unit-tested without a real
// proto value.
type keyAccessor interface {
	GetFromMe() bool
	GetParticipant() string
}

// extractBody pulls the displayable text out of a whatsmeow E2E message
// proto. Reused by the live OnMessage path and the historical
// conversion. Future media-handling will extend this to populate
// MediaPath / MediaType.
func extractBody(m *waE2E.Message) string {
	if m == nil {
		return ""
	}
	if c := m.GetConversation(); c != "" {
		return c
	}
	if e := m.GetExtendedTextMessage(); e != nil {
		return e.GetText()
	}
	return ""
}

// extractQuoted pulls the reply / quoted-message info out of a
// whatsmeow E2E message. Only ExtendedTextMessage carries ContextInfo
// for a text reply; future media-message work will extend this.
//
// Returns ("", "", "") when there's no quote.
func extractQuoted(m *waE2E.Message) (waID, body, sender string) {
	if m == nil {
		return
	}
	ext := m.GetExtendedTextMessage()
	if ext == nil {
		return
	}
	ctx := ext.GetContextInfo()
	if ctx == nil || ctx.GetQuotedMessage() == nil {
		return
	}
	waID = ctx.GetStanzaID()
	sender = ctx.GetParticipant()
	body = extractBody(ctx.GetQuotedMessage())
	return
}

// HistoryPersister extends Persister with the bulk + chat-upsert API
// the history sync handler needs. Defined here so fake stores in the
// wa package can stub it without dragging in store internals.
type HistoryPersister interface {
	Persister
	InsertBatch(ctx context.Context, msgs []store.Message) (int, error)
	UpsertChat(ctx context.Context, jid, name string) error
}

// OnHistorySync persists a HistorySync event: bulk-inserts the
// messages, upserts each conversation's display name (preferring the
// Conversation.Name, falling back to the push-name table for 1:1
// chats).
//
// Cheap on the whatsmeow goroutine: the heavy work is the single
// store.InsertBatch transaction. UI is notified once at the end so
// chat list / message pane both refresh in one frame.
func (h *Handler) OnHistorySync(ctx context.Context, data *waHistorySync.HistorySync, ownJID string) error {
	if h == nil {
		return fmt.Errorf("wa.Handler.OnHistorySync: nil handler")
	}
	if data == nil {
		return nil
	}
	store, ok := h.Store.(HistoryPersister)
	if !ok {
		return fmt.Errorf("wa.Handler.OnHistorySync: Store does not implement HistoryPersister")
	}

	result := fromWMHistorySync(data, ownJID)

	if _, err := store.InsertBatch(ctx, result.Messages); err != nil {
		return fmt.Errorf("wa.Handler.OnHistorySync: persist messages: %w", err)
	}

	for _, c := range result.Chats {
		name := c.Name
		if name == "" {
			// 1:1 chat with no explicit name → fall back to the push
			// name for the JID, if we have one.
			if pn, ok := result.PushNames[c.JID]; ok {
				name = pn
			}
		}
		if name == "" {
			continue
		}
		if err := store.UpsertChat(ctx, c.JID, name); err != nil {
			return fmt.Errorf("wa.Handler.OnHistorySync: upsert chat %s: %w", c.JID, err)
		}
	}

	if h.Notify != nil {
		h.Notify()
	}
	return nil
}
