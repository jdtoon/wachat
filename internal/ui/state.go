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
type State struct {
	store *store.Store

	Chats        []ChatSummary
	SelectedChat string
	Messages     []store.Message // newest-first
	Cursor       store.Cursor    // next page cursor; zero = nothing loaded
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
	return nil
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
//     onto Messages so the new bubble appears immediately. Dedup on WAID.
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
