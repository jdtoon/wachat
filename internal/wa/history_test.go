package wa

import (
	"context"
	"testing"

	"github.com/jdtoon/wachat/internal/store"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"google.golang.org/protobuf/proto"
)

// strPtr / boolPtr / u64Ptr are small helpers so the test fixtures
// read naturally without `proto.String("...")` everywhere.
func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func u64Ptr(v uint64) *uint64 { return &v }

// makeHistorySync builds a HistorySync proto fixture with two chats
// and a few messages — enough to exercise the conversion edges.
func makeHistorySync() *waHistorySync.HistorySync {
	return &waHistorySync.HistorySync{
		SyncType: waHistorySync.HistorySync_FULL.Enum(),
		Conversations: []*waHistorySync.Conversation{
			{
				ID:   strPtr("alice@s.whatsapp.net"),
				Name: strPtr("Alice"),
				Messages: []*waHistorySync.HistorySyncMsg{
					{Message: &waWeb.WebMessageInfo{
						Key: &waCommon.MessageKey{
							ID:        strPtr("msg-a1"),
							RemoteJID: strPtr("alice@s.whatsapp.net"),
							FromMe:    boolPtr(false),
						},
						Message:          &waE2E.Message{Conversation: proto.String("hi there")},
						MessageTimestamp: u64Ptr(1700_000_000),
					}},
					{Message: &waWeb.WebMessageInfo{
						Key: &waCommon.MessageKey{
							ID:        strPtr("msg-a2"),
							RemoteJID: strPtr("alice@s.whatsapp.net"),
							FromMe:    boolPtr(true),
						},
						Message:          &waE2E.Message{Conversation: proto.String("hey")},
						MessageTimestamp: u64Ptr(1700_000_060),
					}},
				},
			},
			{
				ID:   strPtr("group@g.us"),
				Name: strPtr("Engineering"),
				Messages: []*waHistorySync.HistorySyncMsg{
					{Message: &waWeb.WebMessageInfo{
						Key: &waCommon.MessageKey{
							ID:          strPtr("msg-g1"),
							RemoteJID:   strPtr("group@g.us"),
							FromMe:      boolPtr(false),
							Participant: strPtr("bob@s.whatsapp.net"),
						},
						Message:          &waE2E.Message{Conversation: proto.String("ship it")},
						MessageTimestamp: u64Ptr(1700_000_100),
					}},
				},
			},
		},
		Pushnames: []*waHistorySync.Pushname{
			{ID: strPtr("alice@s.whatsapp.net"), Pushname: strPtr("Alice Lovelace")},
			{ID: strPtr("bob@s.whatsapp.net"), Pushname: strPtr("Bob Carter")},
		},
	}
}

func TestFromWMHistorySync_FlatMessageList(t *testing.T) {
	res := fromWMHistorySync(makeHistorySync(), "self@s.whatsapp.net")
	if got := len(res.Messages); got != 3 {
		t.Fatalf("messages = %d, want 3", got)
	}
}

func TestFromWMHistorySync_ChatsCarryName(t *testing.T) {
	res := fromWMHistorySync(makeHistorySync(), "")
	if len(res.Chats) != 2 {
		t.Fatalf("chats = %d, want 2", len(res.Chats))
	}
	want := map[string]string{
		"alice@s.whatsapp.net": "Alice",
		"group@g.us":           "Engineering",
	}
	for _, c := range res.Chats {
		if want[c.JID] != c.Name {
			t.Errorf("chat %s: name=%q, want %q", c.JID, c.Name, want[c.JID])
		}
	}
}

func TestFromWMHistorySync_FromMeUsesOwnJID(t *testing.T) {
	res := fromWMHistorySync(makeHistorySync(), "self@s.whatsapp.net")
	var ownMsg store.Message
	for _, m := range res.Messages {
		if m.WAID == "msg-a2" {
			ownMsg = m
		}
	}
	if ownMsg.WAID == "" {
		t.Fatal("msg-a2 missing")
	}
	if ownMsg.SenderJID != "self@s.whatsapp.net" {
		t.Errorf("from-me sender = %q, want self JID", ownMsg.SenderJID)
	}
}

func TestFromWMHistorySync_GroupSenderUsesParticipant(t *testing.T) {
	res := fromWMHistorySync(makeHistorySync(), "self@s.whatsapp.net")
	var gMsg store.Message
	for _, m := range res.Messages {
		if m.WAID == "msg-g1" {
			gMsg = m
		}
	}
	if gMsg.SenderJID != "bob@s.whatsapp.net" {
		t.Errorf("group sender = %q, want bob@s.whatsapp.net", gMsg.SenderJID)
	}
}

func TestFromWMHistorySync_OneOnOneSenderUsesChatJID(t *testing.T) {
	res := fromWMHistorySync(makeHistorySync(), "self@s.whatsapp.net")
	var rcvd store.Message
	for _, m := range res.Messages {
		if m.WAID == "msg-a1" {
			rcvd = m
		}
	}
	if rcvd.SenderJID != "alice@s.whatsapp.net" {
		t.Errorf("1:1 received sender = %q, want chat JID", rcvd.SenderJID)
	}
}

func TestFromWMHistorySync_TimestampInMillis(t *testing.T) {
	res := fromWMHistorySync(makeHistorySync(), "")
	for _, m := range res.Messages {
		if m.TS < 1_500_000_000_000 || m.TS > 2_000_000_000_000 {
			t.Errorf("msg %s TS=%d not in millisecond range", m.WAID, m.TS)
		}
	}
}

func TestFromWMHistorySync_PushNames(t *testing.T) {
	res := fromWMHistorySync(makeHistorySync(), "")
	if got := res.PushNames["alice@s.whatsapp.net"]; got != "Alice Lovelace" {
		t.Errorf("alice push name = %q, want Alice Lovelace", got)
	}
	if got := res.PushNames["bob@s.whatsapp.net"]; got != "Bob Carter" {
		t.Errorf("bob push name = %q, want Bob Carter", got)
	}
}

func TestFromWMHistorySync_NilDataIsSafe(t *testing.T) {
	res := fromWMHistorySync(nil, "")
	if len(res.Messages) != 0 || len(res.Chats) != 0 {
		t.Errorf("nil data should produce empty result, got %+v", res)
	}
}

// fakeKey implements keyAccessor so senderFromKey can be unit-tested
// without building a full *waCommon.MessageKey.
type fakeKey struct {
	fromMe      bool
	participant string
}

func (f fakeKey) GetFromMe() bool        { return f.fromMe }
func (f fakeKey) GetParticipant() string { return f.participant }

func TestSenderFromKey_TruthTable(t *testing.T) {
	cases := []struct {
		name, chatJID, ownJID string
		key                   fakeKey
		want                  string
	}{
		{
			name: "from me", chatJID: "alice@s.whatsapp.net", ownJID: "self@s.whatsapp.net",
			key:  fakeKey{fromMe: true},
			want: "self@s.whatsapp.net",
		},
		{
			name: "group participant", chatJID: "group@g.us", ownJID: "self@s.whatsapp.net",
			key:  fakeKey{participant: "bob@s.whatsapp.net"},
			want: "bob@s.whatsapp.net",
		},
		{
			name: "1:1 received", chatJID: "alice@s.whatsapp.net", ownJID: "self@s.whatsapp.net",
			key:  fakeKey{},
			want: "alice@s.whatsapp.net",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := senderFromKey(tc.key, tc.chatJID, tc.ownJID); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// fakeHistoryPersister satisfies HistoryPersister so OnHistorySync can
// be tested without spinning up a real *store.Store.
type fakeHistoryPersister struct {
	inserts []store.Message
	batchN  int
	upserts map[string]string
}

func newFakeHistoryPersister() *fakeHistoryPersister {
	return &fakeHistoryPersister{upserts: make(map[string]string)}
}

func (f *fakeHistoryPersister) Insert(_ context.Context, m store.Message, _ bool) (bool, error) {
	f.inserts = append(f.inserts, m)
	return true, nil
}

func (f *fakeHistoryPersister) InsertBatch(_ context.Context, msgs []store.Message) (int, error) {
	f.inserts = append(f.inserts, msgs...)
	f.batchN += len(msgs)
	return len(msgs), nil
}

func (f *fakeHistoryPersister) UpsertChat(_ context.Context, jid, name string) error {
	f.upserts[jid] = name
	return nil
}

func TestHandler_OnHistorySync_PersistsMessagesAndUpsertsChats(t *testing.T) {
	fp := newFakeHistoryPersister()
	h := &Handler{Store: fp}

	if err := h.OnHistorySync(context.Background(), makeHistorySync(), "self@s.whatsapp.net"); err != nil {
		t.Fatalf("OnHistorySync: %v", err)
	}

	if fp.batchN != 3 {
		t.Errorf("InsertBatch received %d messages, want 3", fp.batchN)
	}
	if fp.upserts["alice@s.whatsapp.net"] != "Alice" {
		t.Errorf("alice upsert name = %q, want Alice", fp.upserts["alice@s.whatsapp.net"])
	}
	if fp.upserts["group@g.us"] != "Engineering" {
		t.Errorf("group upsert name = %q, want Engineering", fp.upserts["group@g.us"])
	}
}

func TestHandler_OnHistorySync_FallsBackToPushNameForUnnamedChat(t *testing.T) {
	// Conversation without a Name — should pick up the push name.
	sync := &waHistorySync.HistorySync{
		Conversations: []*waHistorySync.Conversation{
			{ID: strPtr("alice@s.whatsapp.net")},
		},
		Pushnames: []*waHistorySync.Pushname{
			{ID: strPtr("alice@s.whatsapp.net"), Pushname: strPtr("Alice")},
		},
	}
	fp := newFakeHistoryPersister()
	h := &Handler{Store: fp}
	if err := h.OnHistorySync(context.Background(), sync, ""); err != nil {
		t.Fatalf("OnHistorySync: %v", err)
	}
	if fp.upserts["alice@s.whatsapp.net"] != "Alice" {
		t.Errorf("expected push-name fallback to Alice, got %q", fp.upserts["alice@s.whatsapp.net"])
	}
}

func TestHandler_OnHistorySync_NilHandlerErrors(t *testing.T) {
	var h *Handler
	if err := h.OnHistorySync(context.Background(), makeHistorySync(), ""); err == nil {
		t.Error("nil handler: want error")
	}
}

func TestHandler_OnHistorySync_NilDataIsNoOp(t *testing.T) {
	fp := newFakeHistoryPersister()
	h := &Handler{Store: fp}
	if err := h.OnHistorySync(context.Background(), nil, ""); err != nil {
		t.Errorf("nil data should be no-op, got error: %v", err)
	}
	if fp.batchN != 0 {
		t.Errorf("nil data persisted %d messages", fp.batchN)
	}
}

func TestHandler_OnHistorySync_PersisterMustImplementHistoryPersister(t *testing.T) {
	// fakePersister (from handler_test.go) only implements Insert, not
	// InsertBatch / UpsertChat. OnHistorySync should fail cleanly
	// rather than panic.
	h := &Handler{Store: &fakePersister{}}
	err := h.OnHistorySync(context.Background(), makeHistorySync(), "")
	if err == nil {
		t.Error("expected error when Store is not a HistoryPersister")
	}
}
