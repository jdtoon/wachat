package wa

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jdtoon/wachat/internal/store"
)

// fakePersister captures Insert calls and can be configured to fail.
type fakePersister struct {
	inserts  []store.Message
	bumps    []bool
	err      error
	onInsert func() // optional hook fired inside Insert
	insertCt atomic.Int32
}

func (f *fakePersister) Insert(_ context.Context, m store.Message, bump bool) (bool, error) {
	f.insertCt.Add(1)
	if f.onInsert != nil {
		f.onInsert()
	}
	if f.err != nil {
		return false, f.err
	}
	f.inserts = append(f.inserts, m)
	f.bumps = append(f.bumps, bump)
	return true, nil
}

func sampleEvent() MessageEvent {
	return MessageEvent{
		WAID:      "w1",
		ChatJID:   "c1@s.whatsapp.net",
		SenderJID: "alice@s.whatsapp.net",
		TS:        1000,
		Body:      "hi",
		FromMe:    false,
	}
}

func TestHandler_OnMessage_PersistsSendsAndNotifies(t *testing.T) {
	fp := &fakePersister{}
	out := make(chan MessageEvent, 1)
	var notifyCount atomic.Int32
	h := &Handler{
		Store:  fp,
		Out:    out,
		Notify: func() { notifyCount.Add(1) },
	}

	if err := h.OnMessage(context.Background(), sampleEvent()); err != nil {
		t.Fatalf("OnMessage: %v", err)
	}

	if got := len(fp.inserts); got != 1 {
		t.Errorf("inserts = %d, want 1", got)
	}
	if got := fp.inserts[0].WAID; got != "w1" {
		t.Errorf("WAID = %q, want w1", got)
	}
	select {
	case ev := <-out:
		if ev.WAID != "w1" {
			t.Errorf("channel WAID = %q, want w1", ev.WAID)
		}
	default:
		t.Error("event not sent on channel")
	}
	if notifyCount.Load() != 1 {
		t.Errorf("notify called %d times, want 1", notifyCount.Load())
	}
}

func TestHandler_OnMessage_FromMeDoesNotBumpUnread(t *testing.T) {
	fp := &fakePersister{}
	h := &Handler{Store: fp}

	ev := sampleEvent()
	ev.FromMe = true
	if err := h.OnMessage(context.Background(), ev); err != nil {
		t.Fatalf("OnMessage: %v", err)
	}

	if len(fp.bumps) != 1 || fp.bumps[0] != false {
		t.Errorf("bumps = %+v, want [false] (FromMe should not bump unread)", fp.bumps)
	}
}

func TestHandler_OnMessage_IncomingBumpsUnread(t *testing.T) {
	fp := &fakePersister{}
	h := &Handler{Store: fp}

	if err := h.OnMessage(context.Background(), sampleEvent()); err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if len(fp.bumps) != 1 || fp.bumps[0] != true {
		t.Errorf("bumps = %+v, want [true]", fp.bumps)
	}
}

func TestHandler_OnMessage_StoreErrorAborts(t *testing.T) {
	want := errors.New("disk full")
	fp := &fakePersister{err: want}
	out := make(chan MessageEvent, 1)
	var notifyCount atomic.Int32
	h := &Handler{
		Store:  fp,
		Out:    out,
		Notify: func() { notifyCount.Add(1) },
	}

	err := h.OnMessage(context.Background(), sampleEvent())
	if err == nil {
		t.Fatal("expected error from OnMessage when Insert fails")
	}
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want wrap of %v", err, want)
	}
	if got := fp.insertCt.Load(); got != 1 {
		t.Errorf("Insert called %d times, want 1", got)
	}
	select {
	case <-out:
		t.Error("channel must not be sent on when Insert errors")
	default:
	}
	if notifyCount.Load() != 0 {
		t.Errorf("Notify called %d times, want 0 when Insert errors", notifyCount.Load())
	}
}

func TestHandler_OnMessage_FullChannelDoesNotBlock(t *testing.T) {
	fp := &fakePersister{}
	out := make(chan MessageEvent, 1)
	out <- sampleEvent() // pre-fill so the next send must drop
	var notifyCount atomic.Int32
	h := &Handler{
		Store:  fp,
		Out:    out,
		Notify: func() { notifyCount.Add(1) },
	}

	done := make(chan error, 1)
	go func() {
		done <- h.OnMessage(context.Background(), sampleEvent())
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("OnMessage with full channel: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("OnMessage blocked on full channel — must be non-blocking")
	}

	if notifyCount.Load() != 1 {
		t.Errorf("Notify called %d times, want 1 (must fire even when channel is full)", notifyCount.Load())
	}
	if len(fp.inserts) != 1 {
		t.Errorf("inserts = %d, want 1 (Insert must run even when channel is full)", len(fp.inserts))
	}
}

func TestHandler_OnMessage_NilStoreReturnsError(t *testing.T) {
	h := &Handler{}
	if err := h.OnMessage(context.Background(), sampleEvent()); err == nil {
		t.Error("OnMessage with nil Store: want error, got nil")
	}
}

func TestHandler_OnMessage_NilHandlerReturnsError(t *testing.T) {
	var h *Handler
	if err := h.OnMessage(context.Background(), sampleEvent()); err == nil {
		t.Error("(nil *Handler).OnMessage: want error, got nil")
	}
}

// TestHandler_OnMessage_InsertBeforeChannelSend asserts the ordering
// contract: when Insert errors, no channel send happens. This is the
// observable consequence of "persist first, then publish".
//
// (Strict timing of the success-path order would be racy to assert; the
// negative-path test above already encodes the same invariant.)
func TestHandler_OnMessage_InsertBeforeChannelSend(t *testing.T) {
	// Already covered by TestHandler_OnMessage_StoreErrorAborts — this
	// stub exists so the contract is explicitly named.
	t.Log("ordering invariant exercised by TestHandler_OnMessage_StoreErrorAborts")
}

// BenchmarkHandler_OnMessage_FullChannel measures the worst-case handler
// latency: full channel, hot store. The number should be in the µs range —
// well under any UI frame budget.
func BenchmarkHandler_OnMessage_FullChannel(b *testing.B) {
	fp := &fakePersister{}
	out := make(chan MessageEvent, 1)
	out <- sampleEvent()
	h := &Handler{
		Store:  fp,
		Out:    out,
		Notify: func() {},
	}
	ctx := context.Background()
	ev := sampleEvent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.OnMessage(ctx, ev)
	}
}
