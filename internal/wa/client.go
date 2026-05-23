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
	// Auto-reconnect on transient network failures is the right
	// default for a desktop chat client — without it the user has to
	// manually re-pair every time their wifi blips. whatsmeow handles
	// the backoff internally.
	wm.EnableAutoReconnect = true
	return &Client{wm: wm, container: container}, nil
}

// PairPhone returns an 8-character pairing code that can be typed into
// the WhatsApp phone app instead of scanning the QR. Caller must have
// already started Connect() — whatsmeow needs the websocket to be open
// before requesting a code.
//
// `phone` is the target's number in international format (no '+', e.g.
// "27821234567"). The returned code is shown to the user; they type it
// in WhatsApp → Linked Devices → Link with phone number.
//
// clientDisplay is the label that appears on the user's phone in the
// linked-devices list — pass e.g. "wachat (Windows)".
func (c *Client) PairPhone(ctx context.Context, phone, clientDisplay string) (string, error) {
	if c == nil || c.wm == nil {
		return "", fmt.Errorf("wa.PairPhone: client is nil")
	}
	if phone == "" {
		return "", fmt.Errorf("wa.PairPhone: phone is required")
	}
	code, err := c.wm.PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, clientDisplay)
	if err != nil {
		return "", fmt.Errorf("wa.PairPhone: %w", err)
	}
	return code, nil
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

// QRItem is the wachat-local view of a whatsmeow QR pairing event. We
// re-export it (rather than passing whatsmeow.QRChannelItem through) so
// callers outside this package never have to import whatsmeow. See
// CLAUDE.md §3 — keeping the boundary thin contains API drift.
type QRItem struct {
	Event string // "code", "success", "timeout", "err-*"
	Code  string // pairing code when Event == "code"
}

// QRChannel returns a stream of QR pairing events. "code" items carry a
// new QR pairing payload in Code; "success" / "timeout" are terminal.
// Call before Connect. The channel closes when whatsmeow finishes (or
// times out) the pairing flow.
func (c *Client) QRChannel(ctx context.Context) (<-chan QRItem, error) {
	raw, err := c.wm.GetQRChannel(ctx)
	if err != nil {
		return nil, fmt.Errorf("wa.QRChannel: %w", err)
	}
	return adaptQRChannel(raw), nil
}

// adaptQRChannel converts whatsmeow's QRChannelItem stream into wachat's
// QRItem stream. Extracted so the conversion (and the goroutine that
// owns the bridge channel) can be unit-tested without dialing WhatsApp.
func adaptQRChannel(raw <-chan whatsmeow.QRChannelItem) <-chan QRItem {
	out := make(chan QRItem, 4)
	go func() {
		defer close(out)
		for item := range raw {
			out <- QRItem{Event: item.Event, Code: item.Code}
		}
	}()
	return out
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
