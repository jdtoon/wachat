package wa

import (
	"context"
	"testing"

	"github.com/jdtoon/wachat/internal/store"

	"go.mau.fi/whatsmeow/types"
)

// fakeStatusStore implements StatusUpdater + the broader Persister
// surface so it can be plugged into Handler.Store.
type fakeStatusStore struct {
	calls []struct {
		waID, status string
	}
}

func (f *fakeStatusStore) Insert(_ context.Context, _ store.Message, _ bool) (bool, error) {
	return true, nil
}
func (f *fakeStatusStore) UpdateStatus(_ context.Context, waID, status string) error {
	f.calls = append(f.calls, struct{ waID, status string }{waID, status})
	return nil
}

func TestReceiptStatus_TruthTable(t *testing.T) {
	cases := []struct {
		kind types.ReceiptType
		want string
	}{
		{types.ReceiptTypeDelivered, store.StatusDelivered},
		{types.ReceiptTypeRead, store.StatusRead},
		{types.ReceiptTypeReadSelf, store.StatusRead},
		{types.ReceiptTypePlayed, store.StatusPlayed},
		{types.ReceiptTypePlayedSelf, store.StatusPlayed},
		{types.ReceiptTypeRetry, ""},
		{types.ReceiptTypeSender, ""},
		{types.ReceiptTypeServerError, ""},
		{types.ReceiptTypeInactive, ""},
		{types.ReceiptTypePeerMsg, ""},
	}
	for _, tc := range cases {
		if got := receiptStatus(tc.kind); got != tc.want {
			t.Errorf("receiptStatus(%q) = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestHandler_OnReceipt_UpdatesEveryID(t *testing.T) {
	fs := &fakeStatusStore{}
	h := &Handler{Store: fs}

	h.OnReceipt(context.Background(),
		[]types.MessageID{"a", "b", "c"},
		types.ReceiptTypeDelivered,
	)
	if len(fs.calls) != 3 {
		t.Fatalf("UpdateStatus calls = %d, want 3", len(fs.calls))
	}
	for _, call := range fs.calls {
		if call.status != store.StatusDelivered {
			t.Errorf("UpdateStatus(%s) status = %q, want delivered", call.waID, call.status)
		}
	}
}

func TestHandler_OnReceipt_IgnoresNoOpKinds(t *testing.T) {
	fs := &fakeStatusStore{}
	h := &Handler{Store: fs}
	h.OnReceipt(context.Background(),
		[]types.MessageID{"a"}, types.ReceiptTypeRetry)
	if len(fs.calls) != 0 {
		t.Errorf("retry receipt should not trigger UpdateStatus; got %d calls", len(fs.calls))
	}
}

func TestHandler_OnReceipt_EmptyIDSkipped(t *testing.T) {
	fs := &fakeStatusStore{}
	h := &Handler{Store: fs}
	h.OnReceipt(context.Background(),
		[]types.MessageID{"", "real-id", ""},
		types.ReceiptTypeRead,
	)
	if len(fs.calls) != 1 || fs.calls[0].waID != "real-id" {
		t.Errorf("expected single call for real-id, got %+v", fs.calls)
	}
}

func TestHandler_OnReceipt_NilHandlerIsSafe(t *testing.T) {
	var h *Handler
	// Must not panic.
	h.OnReceipt(context.Background(), []types.MessageID{"a"}, types.ReceiptTypeRead)
}

func TestHandler_OnReceipt_StoreWithoutStatusUpdaterIsNoOp(t *testing.T) {
	// fakePersister (from handler_test.go) is only an Insert store.
	h := &Handler{Store: &fakePersister{}}
	// Should not panic, should silently skip.
	h.OnReceipt(context.Background(), []types.MessageID{"a"}, types.ReceiptTypeRead)
}
