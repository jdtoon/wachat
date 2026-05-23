// Package ui owns wachat's view-model state and its layout code.
//
// State is the reducer: it is the single source of truth for what the
// frame loop should draw. All mutations happen on the UI goroutine (see
// CLAUDE.md §4 / §8). Background goroutines never touch State directly —
// they enqueue MessageEvents on a channel that the frame loop drains and
// then calls OnIncoming.
package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jdtoon/wachat/internal/store"
	"github.com/jdtoon/wachat/internal/wa"
)

// ChatSummary is one row in the chat list. It is the projection of a
// chats-table row that the chat list view actually renders.
type ChatSummary struct {
	JID    string
	Name   string
	LastTS int64
	Unread int
}

// PageSize is the default number of messages fetched per keyset page.
const PageSize = 50

// State holds the view-model: the chat list, which chat is open, and the
// currently-loaded window of messages with its keyset cursor. The cursor
// is the (TS, ID) of the OLDEST loaded message — use it to fetch the next
// older page (CLAUDE.md §6).
//
// Messages are stored newest-first (Messages[0] is the newest); the
// frame loop iterates them in reverse so the newest renders at the
// BOTTOM of the message pane (WhatsApp convention, docs/design.md §3).
type State struct {
	store *store.Store

	// OwnJID is the JID of the locally paired device. Used by the
	// bubble layout to decide sent (right-aligned) vs received
	// (left-aligned). Empty until pairing completes.
	OwnJID string

	Chats        []ChatSummary
	SelectedChat string
	Messages     []store.Message // newest-first
	Cursor       store.Cursor    // next page cursor; zero = nothing loaded

	// Results is the live FTS5 search hit set rendered in the search
	// overlay. Nil = no active search; empty non-nil = "search ran,
	// no hits."
	Results SearchResults

	// Reactions on the currently loaded message window, indexed by
	// target wa_id. Refreshed alongside Messages on SelectChat /
	// LoadOlder so the bubble layout doesn't hit the DB per row.
	Reactions map[string][]store.Reaction
}

// NewState binds the reducer to a store. The store is required even though
// some methods (OnIncoming) do not touch it — keeping the dependency in
// the constructor surfaces the contract that State is store-backed.
func NewState(s *store.Store) *State { return &State{store: s} }

// LoadChats refreshes the chat list from the store. Sorted newest-first.
func (st *State) LoadChats(ctx context.Context) error {
	rows, err := st.store.DB().QueryContext(ctx,
		`SELECT jid, name, COALESCE(last_ts, 0), unread FROM chats`)
	if err != nil {
		return fmt.Errorf("ui.LoadChats: %w", err)
	}
	defer rows.Close()

	var out []ChatSummary
	for rows.Next() {
		var c ChatSummary
		if err := rows.Scan(&c.JID, &c.Name, &c.LastTS, &c.Unread); err != nil {
			return fmt.Errorf("ui.LoadChats: scan: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("ui.LoadChats: rows: %w", err)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastTS > out[j].LastTS })
	st.Chats = out
	return nil
}

// SelectChat opens a chat and loads its most recent page of messages. A
// no-op if jid already matches SelectedChat.
func (st *State) SelectChat(ctx context.Context, jid string) error {
	if jid == "" {
		return fmt.Errorf("ui.SelectChat: jid is required")
	}
	if jid == st.SelectedChat && st.Messages != nil {
		return nil
	}
	msgs, next, err := st.store.PageOlder(ctx, jid, store.Cursor{}, PageSize)
	if err != nil {
		return fmt.Errorf("ui.SelectChat: %w", err)
	}
	st.SelectedChat = jid
	st.Messages = msgs
	st.Cursor = next
	st.reloadReactions(ctx)
	return nil
}

// reloadReactions refreshes State.Reactions for the currently loaded
// Messages window. Single SQL query (IN-clause batched). Cheap on the
// UI goroutine because the window is bounded (~PageSize × pages
// loaded).
func (st *State) reloadReactions(ctx context.Context) {
	if len(st.Messages) == 0 {
		st.Reactions = nil
		return
	}
	ids := make([]string, 0, len(st.Messages))
	for _, m := range st.Messages {
		if m.WAID != "" {
			ids = append(ids, m.WAID)
		}
	}
	groups, err := st.store.ReactionsForChat(ctx, ids)
	if err != nil {
		return
	}
	st.Reactions = groups
}

// LoadOlder appends the next older page of messages to Messages. Returns
// the number of rows appended; zero means the history is exhausted.
func (st *State) LoadOlder(ctx context.Context) (int, error) {
	if st.SelectedChat == "" {
		return 0, fmt.Errorf("ui.LoadOlder: no chat selected")
	}
	if st.Cursor.IsZero() && len(st.Messages) > 0 {
		// Cursor zero with non-empty Messages = first page was the whole
		// history. Nothing older to fetch.
		return 0, nil
	}
	cursor := st.Cursor
	if cursor.IsZero() && len(st.Messages) == 0 {
		// Nothing loaded yet — same as a SelectChat call.
		msgs, next, err := st.store.PageOlder(ctx, st.SelectedChat, store.Cursor{}, PageSize)
		if err != nil {
			return 0, fmt.Errorf("ui.LoadOlder: %w", err)
		}
		st.Messages = msgs
		st.Cursor = next
		return len(msgs), nil
	}
	msgs, next, err := st.store.PageOlder(ctx, st.SelectedChat, cursor, PageSize)
	if err != nil {
		return 0, fmt.Errorf("ui.LoadOlder: %w", err)
	}
	st.Messages = append(st.Messages, msgs...)
	st.Cursor = next
	st.reloadReactions(ctx)
	return len(msgs), nil
}

// OnIncoming folds a freshly-arrived message into the view-model. It is
// the only state mutation that runs in response to whatsmeow events; the
// frame loop calls it after draining the wa.Handler channel.
//
// Behavior:
//   - Updates (or creates) the chat's ChatSummary row. LastTS only
//     advances; unread bumps for non-FromMe messages.
//   - Re-sorts the chat list (the affected row may have moved to the top).
//   - If the message belongs to the currently selected chat, prepends it
//     onto Messages (which is stored newest-first). Dedup on WAID.
//
// Storage is newest-first but the message pane renders index N-1..0 in
// the layout (so newest sits at the bottom of the viewport) — see
// view.go layoutMessages. The prepend keeps Messages[0] the newest;
// the render code does the visual flip.
//
// Pure function — does not touch the store. The store row was already
// persisted by wa.Handler before this method is invoked.
func (st *State) OnIncoming(ev wa.MessageEvent) {
	st.upsertChatSummary(ev)
	sort.SliceStable(st.Chats, func(i, j int) bool { return st.Chats[i].LastTS > st.Chats[j].LastTS })

	if ev.ChatJID == st.SelectedChat {
		m := store.Message{
			WAID:      ev.WAID,
			ChatJID:   ev.ChatJID,
			SenderJID: ev.SenderJID,
			TS:        ev.TS,
			Body:      ev.Body,
		}
		// Dedup on WAID — whatsmeow can redeliver and we don't want the
		// same bubble appearing twice. If we already had it, replace
		// (the new copy carries any fresher fields).
		for i := range st.Messages {
			if st.Messages[i].WAID == ev.WAID {
				st.Messages[i] = m
				return
			}
		}
		st.Messages = append([]store.Message{m}, st.Messages...)
	}
}

func (st *State) upsertChatSummary(ev wa.MessageEvent) {
	for i := range st.Chats {
		c := &st.Chats[i]
		if c.JID == ev.ChatJID {
			if ev.TS > c.LastTS {
				c.LastTS = ev.TS
			}
			if !ev.FromMe {
				c.Unread++
			}
			return
		}
	}
	unread := 0
	if !ev.FromMe {
		unread = 1
	}
	st.Chats = append(st.Chats, ChatSummary{
		JID:    ev.ChatJID,
		LastTS: ev.TS,
		Unread: unread,
	})
}

// SearchResults holds the most recent full-text search result set. A
// non-nil zero-length slice means "search ran, no hits"; a nil slice
// means "no active search."
type SearchResults []store.SearchHit

// Search runs an FTS5 search and stores the results on State. The UI
// reads State.Results to render the overlay. Empty query clears the
// results.
func (st *State) Search(ctx context.Context, query string) error {
	q := strings.TrimSpace(query)
	if q == "" {
		st.Results = nil
		return nil
	}
	hits, err := st.store.Search(ctx, q, SearchLimit)
	if err != nil {
		return fmt.Errorf("ui.Search: %w", err)
	}
	st.Results = hits
	return nil
}

// SearchLimit is the maximum hit count displayed in the search overlay
// before the user has to refine. 100 keeps the overlay scrollable and
// the FTS5 query under a few ms.
const SearchLimit = 100

// JumpToMessage opens the chat containing the hit and loads a window of
// messages centered on the hit so it appears in the middle of the
// viewport. Uses store.PageAround for the centered window.
func (st *State) JumpToMessage(ctx context.Context, hit store.SearchHit) error {
	if hit.ChatJID == "" {
		return fmt.Errorf("ui.JumpToMessage: empty ChatJID")
	}
	msgs, next, err := st.store.PageAround(ctx, hit.ChatJID, hit.MessageID, PageSize/2, PageSize/2)
	if err != nil {
		return fmt.Errorf("ui.JumpToMessage: %w", err)
	}
	st.SelectedChat = hit.ChatJID
	st.Messages = msgs
	st.Cursor = next
	st.reloadReactions(ctx)
	// Don't clear search results — the user may want to click another hit.
	return nil
}

// AddOptimistic inserts an outgoing message into the view-model and the
// store before the network round-trip completes. The bubble appears
// immediately in a "pending" state — when the matching incoming event
// arrives via wa.Handler, the dedup path in store.Insert + OnIncoming
// reconciles to the server-confirmed row.
//
// waID is whatsmeow's server-assigned message ID; we already have it
// at this point (wa.SendText returned it before the receipt event).
// chatJID is the conversation we sent to. body is the trimmed text.
//
// Side effects:
//   - Persists the message with empty SenderJID (our local convention
//     for "from me" — see view.isFromMe).
//   - Folds the row into the selected-chat Messages via OnIncoming so
//     the bubble appears immediately. unread is NOT bumped.
func (st *State) AddOptimistic(ctx context.Context, waID, chatJID, body string, ts int64) error {
	if chatJID == "" {
		return fmt.Errorf("ui.AddOptimistic: chatJID is required")
	}
	if waID == "" {
		return fmt.Errorf("ui.AddOptimistic: waID is required")
	}
	msg := store.Message{
		WAID:      waID,
		ChatJID:   chatJID,
		SenderJID: "", // "from me" convention
		TS:        ts,
		Body:      body,
		Status:    store.StatusPending,
	}
	if _, err := st.store.Insert(ctx, msg, false); err != nil {
		return fmt.Errorf("ui.AddOptimistic: persist: %w", err)
	}
	// Reuse the OnIncoming reducer so chat list + selected pane both
	// update — pass FromMe=true so unread isn't bumped and chat
	// summary recomputes correctly.
	st.OnIncoming(wa.MessageEvent{
		WAID:    waID,
		ChatJID: chatJID,
		TS:      ts,
		Body:    body,
		FromMe:  true,
	})
	// OnIncoming would have written StatusSent into the in-memory copy
	// it just prepended (default empty Status → unspecified). Fix the
	// pending tag on the leading element so the bubble's tick shows
	// the in-flight state correctly.
	if len(st.Messages) > 0 && st.Messages[0].WAID == waID {
		st.Messages[0].Status = store.StatusPending
	}
	return nil
}

// NameFor returns the display name persisted for jid, or "" if we
// haven't seen one yet. Used by the message bubble code to render
// sender labels in group chats — we look up each sender JID's
// chat-row name (which is populated from push names + history sync).
func (st *State) NameFor(jid string) string {
	for i := range st.Chats {
		if st.Chats[i].JID == jid {
			return st.Chats[i].Name
		}
	}
	return ""
}

// IsGroup reports whether chatJID is a group JID. WhatsApp groups all
// use the @g.us server suffix.
func IsGroup(chatJID string) bool {
	return strings.HasSuffix(chatJID, "@g.us")
}

// MarkSelectedRead clears the unread counter on the selected chat both in
// the store and in the view-model. Called when the user has visibly read
// the messages in the open conversation.
func (st *State) MarkSelectedRead(ctx context.Context) error {
	if st.SelectedChat == "" {
		return nil
	}
	if err := st.store.MarkRead(ctx, st.SelectedChat); err != nil {
		return fmt.Errorf("ui.MarkSelectedRead: %w", err)
	}
	for i := range st.Chats {
		if st.Chats[i].JID == st.SelectedChat {
			st.Chats[i].Unread = 0
			break
		}
	}
	return nil
}
