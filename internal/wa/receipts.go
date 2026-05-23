package wa

import (
	"context"
	"fmt"

	"github.com/jdtoon/wachat/internal/store"

	"go.mau.fi/whatsmeow/types"
)

// StatusUpdater is the subset of *store.Store the receipt handler
// needs. Defined here so fake stores can implement it narrowly.
type StatusUpdater interface {
	UpdateStatus(ctx context.Context, waID, status string) error
}

// OnReceipt applies a whatsmeow events.Receipt to the messages table.
// One event can carry multiple message IDs (whatsmeow batches when
// the phone sends bulk read receipts); we update each row in turn.
//
// Mapping:
//
//	ReceiptTypeDelivered ("") → store.StatusDelivered
//	ReceiptTypeRead             → store.StatusRead
//	ReceiptTypeReadSelf         → store.StatusRead (read on another device)
//	ReceiptTypePlayed           → store.StatusPlayed (voice notes — future)
//	ReceiptTypePlayedSelf       → store.StatusPlayed
//	other types (retry / sender / server-error / inactive / peer_msg) → ignored
//
// Called from whatsmeow's goroutines — keep cheap; only the
// per-message UPDATE hits the DB.
func (h *Handler) OnReceipt(ctx context.Context, ids []types.MessageID, kind types.ReceiptType) {
	if h == nil || len(ids) == 0 {
		return
	}
	status := receiptStatus(kind)
	if status == "" {
		return
	}
	updater, ok := h.Store.(StatusUpdater)
	if !ok {
		return
	}
	for _, id := range ids {
		s := string(id)
		if s == "" {
			continue
		}
		if err := updater.UpdateStatus(ctx, s, status); err != nil && h.Logger != nil {
			h.Logger(fmt.Errorf("wa.OnReceipt: %s: %w", s, err))
		}
	}
	if h.Notify != nil {
		h.Notify()
	}
}

// receiptStatus maps whatsmeow's ReceiptType into our store status
// constants. Returns "" for types that don't change UI state.
func receiptStatus(kind types.ReceiptType) string {
	switch kind {
	case types.ReceiptTypeDelivered:
		return store.StatusDelivered
	case types.ReceiptTypeRead, types.ReceiptTypeReadSelf:
		return store.StatusRead
	case types.ReceiptTypePlayed, types.ReceiptTypePlayedSelf:
		return store.StatusPlayed
	}
	return ""
}
