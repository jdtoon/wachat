// Package wa wraps whatsmeow (the WhatsApp multidevice protocol library)
// with the thin surface area wachat actually uses: connect, pair, register
// an event handler.
//
// The wrapper exists so the rest of wachat does not depend on whatsmeow's
// types directly — that boundary is where we keep API drift (CLAUDE.md §3)
// from rippling through the codebase.
//
// All construction takes a context per whatsmeow's current API (verified
// against pkg.go.dev/go.mau.fi/whatsmeow at write time — re-verify if you
// see compilation drift).
package wa

import (
	"context"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go SQLite driver registered with database/sql

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Client is wachat's view of the WhatsApp protocol. It owns the whatsmeow
// client and the sqlstore.Container (whatsmeow's session/device state —
// distinct from wachat's own message store).
type Client struct {
	wm        *whatsmeow.Client
	container *sqlstore.Container
}

// New opens the session SQLite container at dbPath, loads (or creates) the
// first device, and constructs the whatsmeow client. It does not connect.
//
// The container file is separate from wachat's message store so the two
// concerns stay isolated and either can be wiped independently.
func New(ctx context.Context, dbPath string) (*Client, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", dbPath)
	container, err := sqlstore.New(ctx, "sqlite", dsn, waLog.Noop)
	if err != nil {
		return nil, fmt.Errorf("wa.New: open sqlstore at %q: %w", dbPath, err)
	}

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("wa.New: GetFirstDevice: %w", err)
	}

	wm := whatsmeow.NewClient(device, waLog.Noop)
	return &Client{wm: wm, container: container}, nil
}

// NeedsPairing reports whether the device has not yet been linked to a
// phone. Callers should display the QR pairing flow (QRChannel + Connect)
// when this returns true.
func (c *Client) NeedsPairing() bool {
	if c == nil || c.wm == nil || c.wm.Store == nil {
		return true
	}
	return c.wm.Store.ID == nil
}

// QRChannel returns the whatsmeow pairing QR stream. Items have an Event
// field: "code" carries a new QR code in Code; QRChannelSuccess /
// QRChannelTimeout / err-* signal terminal states. Call before Connect.
func (c *Client) QRChannel(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error) {
	ch, err := c.wm.GetQRChannel(ctx)
	if err != nil {
		return nil, fmt.Errorf("wa.QRChannel: %w", err)
	}
	return ch, nil
}

// Connect dials the WhatsApp servers. If NeedsPairing returns true, the
// caller must already be draining QRChannel so the codes can be displayed.
func (c *Client) Connect() error {
	if err := c.wm.Connect(); err != nil {
		return fmt.Errorf("wa.Connect: %w", err)
	}
	return nil
}

// Disconnect closes the network connection. Safe to call on a Client that
// never Connected.
func (c *Client) Disconnect() {
	if c == nil || c.wm == nil {
		return
	}
	c.wm.Disconnect()
}

// AddEventHandler registers h to receive whatsmeow events. Returns the
// handler id for later removal. The handler is invoked on whatsmeow's
// background goroutines — keep it cheap and never mutate UI state from it
// (CLAUDE.md §4).
func (c *Client) AddEventHandler(h whatsmeow.EventHandler) uint32 {
	return c.wm.AddEventHandler(h)
}

// Close disconnects (if needed) and releases the session container. It is
// safe to call on a Client that never Connected.
func (c *Client) Close() error {
	c.Disconnect()
	if c.container != nil {
		if err := c.container.Close(); err != nil {
			return fmt.Errorf("wa.Close: close container: %w", err)
		}
	}
	return nil
}
