package wa

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

// EditRevoker is the subset of *store.Store the edit/revoke handler
// needs. Stays narrow so fake stores can stub it cheaply.
type EditRevoker interface {
	ApplyEdit(ctx context.Context, waID, newBody string) error
	ApplyRevoke(ctx context.Context, waID string) error
}

// handleProtocol dispatches a ProtocolMessage embedded inside a
// regular events.Message. Currently handles two cases:
//
//   - MESSAGE_EDIT: the sender edited an earlier message. We replace
//     the body via store.ApplyEdit and the UI re-renders with the new
//     text + an "(edited)" marker.
//   - REVOKE: the sender used "delete for everyone." store.ApplyRevoke
//     flips the revoked flag; the UI renders "[message deleted]"
//     instead of the body.
//
// Other ProtocolMessage types (history sync notification, app state
// sync key, ephemeral settings, ...) are handled elsewhere or ignored.
func (h *Handler) handleProtocol(ctx context.Context, pm *waE2E.ProtocolMessage) {
	if h == nil || pm == nil {
		return
	}
	editStore, ok := h.Store.(EditRevoker)
	if !ok {
		return
	}
	key := pm.GetKey()
	if key == nil {
		return
	}
	waID := key.GetID()
	if waID == "" {
		return
	}

	switch pm.GetType() {
	case waE2E.ProtocolMessage_REVOKE:
		if err := editStore.ApplyRevoke(ctx, waID); err != nil && h.Logger != nil {
			h.Logger(fmt.Errorf("wa.handleProtocol: revoke %s: %w", waID, err))
		}
	case waE2E.ProtocolMessage_MESSAGE_EDIT:
		newBody := extractBody(pm.GetEditedMessage())
		if err := editStore.ApplyEdit(ctx, waID, newBody); err != nil && h.Logger != nil {
			h.Logger(fmt.Errorf("wa.handleProtocol: edit %s: %w", waID, err))
		}
	default:
		return
	}
	if h.Notify != nil {
		h.Notify()
	}
}
