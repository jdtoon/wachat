package ui

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jdtoon/wachat/internal/store"
	"github.com/jdtoon/wachat/internal/wa"
)

func openState(t *testing.T) (*State, *store.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "wachat.db")
	s, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return NewState(s), s
}

func mustInsert(t *testing.T, s *store.Store, m store.Message, bump bool) {
	t.Helper()
	if _, err := s.Insert(context.Background(), m, bump); err != nil {
		t.Fatalf("store.Insert(%s): %v", m.WAID, err)
	}
}

// --- OnIncoming (pure reducer) ---

func TestOnIncoming_AppendsNewChat(t *testing.T) {
	st, _ := openState(t)

	st.OnIncoming(wa.MessageEvent{WAID: "w1", ChatJID: "c1", TS: 100})

	if len(st.Chats) != 1 {
		t.Fatalf("Chats len = %d, want 1", len(st.Chats))
	}
	got := st.Chats[0]
	if got.JID != "c1" || got.LastTS != 100 || got.Unread != 1 {
		t.Errorf("Chats[0] = %+v, want {c1, 100, 1}", got)
	}
}

func TestOnIncoming_FromMeDoesNotBumpUnread(t *testing.T) {
	st, _ := openState(t)

	st.OnIncoming(wa.MessageEvent{WAID: "w1", ChatJID: "c1", TS: 100, FromMe: true})

	if st.Chats[0].Unread != 0 {
		t.Errorf("Unread = %d, want 0 (FromMe)", st.Chats[0].Unread)
	}
}

func TestOnIncoming_UpdatesExistingChatAndResorts(t *testing.T) {
	st, _ := openState(t)

	st.OnIncoming(wa.MessageEvent{WAID: "w1", ChatJID: "c1", TS: 100})
	st.OnIncoming(wa.MessageEvent{WAID: "w2", ChatJID: "c2", TS: 200})
	// Newer message in c1 should bubble it to the top.
	st.OnIncoming(wa.MessageEvent{WAID: "w3", ChatJID: "c1", TS: 300})

	if len(st.Chats) != 2 {
		t.Fatalf("Chats len = %d, want 2", len(st.Chats))
	}
	if st.Chats[0].JID != "c1" {
		t.Errorf("top chat = %q, want c1 (newest LastTS)", st.Chats[0].JID)
	}
	if st.Chats[0].LastTS != 300 {
		t.Errorf("c1.LastTS = %d, want 300", st.Chats[0].LastTS)
	}
	if st.Chats[0].Unread != 2 {
		t.Errorf("c1.Unread = %d, want 2", st.Chats[0].Unread)
	}
}

func TestOnIncoming_OldMessageDoesNotRegressLastTS(t *testing.T) {
	st, _ := openState(t)

	st.OnIncoming(wa.MessageEvent{WAID: "w1", ChatJID: "c1", TS: 200})
	st.OnIncoming(wa.MessageEvent{WAID: "w2", ChatJID: "c1", TS: 100})

	if st.Chats[0].LastTS != 200 {
		t.Errorf("LastTS = %d, want 200 (older arrival must not regress)", st.Chats[0].LastTS)
	}
}

func TestOnIncoming_PrependsOnSelectedChat(t *testing.T) {
	st, _ := openState(t)
	st.SelectedChat = "c1"

	st.OnIncoming(wa.MessageEvent{WAID: "w1", ChatJID: "c1", TS: 100, Body: "hi"})
	st.OnIncoming(wa.MessageEvent{WAID: "w2", ChatJID: "c1", TS: 200, Body: "again"})

	if len(st.Messages) != 2 {
		t.Fatalf("Messages len = %d, want 2", len(st.Messages))
	}
	if st.Messages[0].WAID != "w2" {
		t.Errorf("Messages[0] = %q, want w2 (newest at front)", st.Messages[0].WAID)
	}
}

func TestOnIncoming_DoesNotPrependOnUnselectedChat(t *testing.T) {
	st, _ := openState(t)
	st.SelectedChat = "c1"

	st.OnIncoming(wa.MessageEvent{WAID: "w1", ChatJID: "OTHER", TS: 100})

	if len(st.Messages) != 0 {
		t.Errorf("Messages len = %d, want 0 (message was for a different chat)", len(st.Messages))
	}
}

func TestOnIncoming_DedupsOnWAID(t *testing.T) {
	st, _ := openState(t)
	st.SelectedChat = "c1"

	st.OnIncoming(wa.MessageEvent{WAID: "w1", ChatJID: "c1", TS: 100, Body: "first"})
	st.OnIncoming(wa.MessageEvent{WAID: "w1", ChatJID: "c1", TS: 100, Body: "redelivered"})

	if len(st.Messages) != 1 {
		t.Errorf("Messages len = %d, want 1 (dedup on WAID)", len(st.Messages))
	}
	if st.Messages[0].Body != "redelivered" {
		t.Errorf("Body = %q, want redelivered (replacement should win)", st.Messages[0].Body)
	}
}

// --- Store-backed loaders ---

func TestLoadChats_SortsByLastTSDesc(t *testing.T) {
	st, s := openState(t)
	ctx := context.Background()

	mustInsert(t, s, store.Message{WAID: "w1", ChatJID: "c1", TS: 100}, true)
	mustInsert(t, s, store.Message{WAID: "w2", ChatJID: "c2", TS: 300}, true)
	mustInsert(t, s, store.Message{WAID: "w3", ChatJID: "c3", TS: 200}, true)

	if err := st.LoadChats(ctx); err != nil {
		t.Fatalf("LoadChats: %v", err)
	}
	want := []string{"c2", "c3", "c1"}
	for i, w := range want {
		if i >= len(st.Chats) || st.Chats[i].JID != w {
			t.Errorf("Chats[%d] = %v, want %s", i, st.Chats, w)
		}
	}
}

func TestSelectChat_LoadsNewestPage(t *testing.T) {
	st, s := openState(t)
	ctx := context.Background()

	for i := 0; i < 60; i++ {
		mustInsert(t, s, store.Message{
			WAID: fmtWA(i), ChatJID: "c1", TS: int64(i + 1),
		}, false)
	}

	if err := st.SelectChat(ctx, "c1"); err != nil {
		t.Fatalf("SelectChat: %v", err)
	}
	if got := len(st.Messages); got != PageSize {
		t.Errorf("Messages len = %d, want %d", got, PageSize)
	}
	if st.SelectedChat != "c1" {
		t.Errorf("SelectedChat = %q, want c1", st.SelectedChat)
	}
	if st.Messages[0].TS != 60 {
		t.Errorf("newest TS = %d, want 60", st.Messages[0].TS)
	}
}

func TestSelectChat_SameChatIsNoOp(t *testing.T) {
	st, s := openState(t)
	ctx := context.Background()
	mustInsert(t, s, store.Message{WAID: "w1", ChatJID: "c1", TS: 1}, false)

	if err := st.SelectChat(ctx, "c1"); err != nil {
		t.Fatalf("SelectChat: %v", err)
	}
	// Mutate Messages to detect whether the second call reloads.
	st.Messages[0].Body = "MUTATED"
	if err := st.SelectChat(ctx, "c1"); err != nil {
		t.Fatalf("SelectChat #2: %v", err)
	}
	if st.Messages[0].Body != "MUTATED" {
		t.Errorf("SelectChat with same jid reloaded; want no-op")
	}
}

func TestSelectChat_RequiresJID(t *testing.T) {
	st, _ := openState(t)
	if err := st.SelectChat(context.Background(), ""); err == nil {
		t.Error("empty jid: want error")
	}
}

func TestLoadOlder_AppendsNextPageAndAdvancesCursor(t *testing.T) {
	st, s := openState(t)
	ctx := context.Background()

	for i := 0; i < 125; i++ {
		mustInsert(t, s, store.Message{
			WAID: fmtWA(i), ChatJID: "c1", TS: int64(i + 1),
		}, false)
	}

	if err := st.SelectChat(ctx, "c1"); err != nil {
		t.Fatalf("SelectChat: %v", err)
	}
	startCursor := st.Cursor

	n, err := st.LoadOlder(ctx)
	if err != nil {
		t.Fatalf("LoadOlder: %v", err)
	}
	if n != PageSize {
		t.Errorf("LoadOlder returned %d, want %d", n, PageSize)
	}
	if st.Cursor == startCursor {
		t.Errorf("cursor did not advance")
	}
	if got := len(st.Messages); got != 2*PageSize {
		t.Errorf("Messages len after LoadOlder = %d, want %d", got, 2*PageSize)
	}

	// One more — partial page (125 - 100 = 25)
	n, err = st.LoadOlder(ctx)
	if err != nil {
		t.Fatalf("LoadOlder #2: %v", err)
	}
	if n != 25 {
		t.Errorf("partial page = %d, want 25", n)
	}
	// History exhausted now.
	n, err = st.LoadOlder(ctx)
	if err != nil {
		t.Fatalf("LoadOlder #3: %v", err)
	}
	if n != 0 {
		t.Errorf("exhausted page = %d, want 0", n)
	}
}

func TestLoadOlder_NoChatSelectedErrors(t *testing.T) {
	st, _ := openState(t)
	if _, err := st.LoadOlder(context.Background()); err == nil {
		t.Error("LoadOlder with no chat selected: want error")
	}
}

func TestMarkSelectedRead_ClearsUnreadInStoreAndState(t *testing.T) {
	st, s := openState(t)
	ctx := context.Background()

	mustInsert(t, s, store.Message{WAID: "w1", ChatJID: "c1", TS: 1}, true)
	mustInsert(t, s, store.Message{WAID: "w2", ChatJID: "c1", TS: 2}, true)
	if err := st.LoadChats(ctx); err != nil {
		t.Fatalf("LoadChats: %v", err)
	}
	if err := st.SelectChat(ctx, "c1"); err != nil {
		t.Fatalf("SelectChat: %v", err)
	}

	if err := st.MarkSelectedRead(ctx); err != nil {
		t.Fatalf("MarkSelectedRead: %v", err)
	}
	if st.Chats[0].Unread != 0 {
		t.Errorf("in-memory unread = %d, want 0", st.Chats[0].Unread)
	}

	// Confirm the DB row was updated too.
	var dbUnread int
	if err := s.DB().QueryRow("SELECT unread FROM chats WHERE jid=?", "c1").Scan(&dbUnread); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if dbUnread != 0 {
		t.Errorf("DB unread = %d, want 0", dbUnread)
	}
}

func TestMarkSelectedRead_NoChatSelectedIsNoOp(t *testing.T) {
	st, _ := openState(t)
	if err := st.MarkSelectedRead(context.Background()); err != nil {
		t.Errorf("no-op MarkSelectedRead errored: %v", err)
	}
}

// --- AddOptimistic (outgoing) ---

func TestAddOptimistic_PersistsAndAppearsInSelectedChat(t *testing.T) {
	st, _ := openState(t)
	st.SelectedChat = "c1"
	ctx := context.Background()

	if err := st.AddOptimistic(ctx, "wa-out-1", "c1", "hello", 1000); err != nil {
		t.Fatalf("AddOptimistic: %v", err)
	}

	if len(st.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(st.Messages))
	}
	if st.Messages[0].WAID != "wa-out-1" {
		t.Errorf("Messages[0].WAID = %q, want wa-out-1", st.Messages[0].WAID)
	}
	if st.Messages[0].SenderJID != "" {
		t.Errorf("Messages[0].SenderJID = %q, want empty (from me)", st.Messages[0].SenderJID)
	}
	if st.Messages[0].Body != "hello" {
		t.Errorf("Messages[0].Body = %q, want hello", st.Messages[0].Body)
	}
}

func TestAddOptimistic_DoesNotBumpUnread(t *testing.T) {
	st, _ := openState(t)
	st.SelectedChat = "c1"

	if err := st.AddOptimistic(context.Background(), "wa-out-1", "c1", "hi", 1000); err != nil {
		t.Fatalf("AddOptimistic: %v", err)
	}
	for _, c := range st.Chats {
		if c.JID == "c1" && c.Unread != 0 {
			t.Errorf("c1.Unread = %d, want 0 (outgoing should not bump unread)", c.Unread)
		}
	}
}

func TestAddOptimistic_ValidatesArgs(t *testing.T) {
	st, _ := openState(t)
	ctx := context.Background()
	if err := st.AddOptimistic(ctx, "", "c1", "body", 1); err == nil {
		t.Error("empty waID: want error")
	}
	if err := st.AddOptimistic(ctx, "w1", "", "body", 1); err == nil {
		t.Error("empty chatJID: want error")
	}
}

// --- Search + JumpToMessage ---

func TestState_SearchPopulatesResults(t *testing.T) {
	st, s := openState(t)
	ctx := context.Background()
	mustInsert(t, s, store.Message{
		WAID: "w1", ChatJID: "c1", TS: 1000, Body: "the quick brown fox",
	}, false)
	mustInsert(t, s, store.Message{
		WAID: "w2", ChatJID: "c1", TS: 1001, Body: "lazy dog",
	}, false)

	if err := st.Search(ctx, "quick"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(st.Results) != 1 {
		t.Errorf("Results = %+v, want 1 hit", st.Results)
	}
}

func TestState_SearchEmptyQueryClears(t *testing.T) {
	st, _ := openState(t)
	st.Results = make(SearchResults, 5) // simulate prior search

	if err := st.Search(context.Background(), "   "); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if st.Results != nil {
		t.Errorf("empty query should null Results; got %v", st.Results)
	}
}

func TestState_JumpToMessageLoadsWindowCenteredOnAnchor(t *testing.T) {
	st, s := openState(t)
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		mustInsert(t, s, store.Message{
			WAID: fmtWA(i), ChatJID: "c1", TS: int64(i + 1), Body: "msg",
		}, false)
	}
	// Find the row id of the 10th-inserted message.
	var anchorID int64
	if err := s.DB().QueryRow(
		`SELECT id FROM messages WHERE wa_id=?`, fmtWA(10),
	).Scan(&anchorID); err != nil {
		t.Fatalf("scan: %v", err)
	}

	hit := store.SearchHit{ChatJID: "c1", MessageID: anchorID}
	if err := st.JumpToMessage(ctx, hit); err != nil {
		t.Fatalf("JumpToMessage: %v", err)
	}
	if st.SelectedChat != "c1" {
		t.Errorf("SelectedChat = %q, want c1", st.SelectedChat)
	}
	hasAnchor := false
	for _, m := range st.Messages {
		if m.ID == anchorID {
			hasAnchor = true
		}
	}
	if !hasAnchor {
		t.Error("anchor not in loaded window after JumpToMessage")
	}
}

func TestState_JumpToMessageEmptyChatJIDErrors(t *testing.T) {
	st, _ := openState(t)
	if err := st.JumpToMessage(context.Background(), store.SearchHit{}); err == nil {
		t.Error("empty ChatJID: want error")
	}
}

func TestAddOptimistic_DedupOnRedelivery(t *testing.T) {
	st, _ := openState(t)
	st.SelectedChat = "c1"
	ctx := context.Background()

	if err := st.AddOptimistic(ctx, "w1", "c1", "v1", 1000); err != nil {
		t.Fatalf("first add: %v", err)
	}
	// Same waID again — must not double the bubble.
	if err := st.AddOptimistic(ctx, "w1", "c1", "v2", 1000); err != nil {
		t.Fatalf("redelivery add: %v", err)
	}
	if len(st.Messages) != 1 {
		t.Errorf("Messages len = %d after redelivery, want 1", len(st.Messages))
	}
}

// fmtWA generates a sortable wa_id for tests.
func fmtWA(i int) string {
	return fmtPad("w", i)
}

func fmtPad(prefix string, i int) string {
	const pad = "00000"
	s := itoa(i)
	if len(s) < len(pad) {
		s = pad[len(s):] + s
	}
	return prefix + s
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
