// Command wachat is a lean, native desktop WhatsApp client.
//
// main wires the store, the wa boundary, the UI view-model, and the Gio
// frame loop together. The frame loop is the single goroutine that owns
// all UI state (CLAUDE.md §4 / §8); everything else (DB writes, network,
// QR pairing, future media decode) runs on background goroutines and
// hands events off via a buffered channel + w.Invalidate().
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/mdp/qrterminal/v3"

	"github.com/jdtoon/wachat/internal/store"
	"github.com/jdtoon/wachat/internal/ui"
	"github.com/jdtoon/wachat/internal/wa"
)

// Version is the current version of wachat. Set via -ldflags at build time
// for release builds; defaults to "dev" for local builds.
var Version = "dev"

// channel buffer for wa.Handler → frame-loop handoff. 64 is generous;
// the actual high-water mark in normal use is single-digit. A full
// channel is not an error — wa.Handler drops the payload (the store row
// is already written) and the next frame re-reads from the store.
const incomingBuffer = 64

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	dbPath := flag.String("db", "wachat.db", "path to the wachat SQLite database")
	noConnect := flag.Bool("no-connect", false, "open the UI without connecting to WhatsApp (offline dev mode)")
	flag.Parse()

	if *showVersion {
		fmt.Println("wachat", Version)
		return
	}

	go func() {
		if err := run(*dbPath, *noConnect); err != nil {
			log.Println("wachat:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(dbPath string, noConnect bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := store.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = s.Close() }()

	state := ui.NewState(s)
	if err := state.LoadChats(ctx); err != nil {
		return fmt.Errorf("load chats: %w", err)
	}
	view := ui.NewView()

	// Window. Title + reasonable default size. Resizable; no MinSize for now.
	w := new(app.Window)
	w.Option(
		app.Title("wachat"),
		app.Size(unit.Dp(900), unit.Dp(600)),
	)

	// Background-goroutine → UI-goroutine handoff channel.
	incoming := make(chan wa.MessageEvent, incomingBuffer)

	// whatsmeow client + handler. Skipped in -no-connect mode so the UI
	// can be exercised offline against the local store.
	if !noConnect {
		sessionPath := sessionDBPath(dbPath)
		waCli, err := wa.New(ctx, sessionPath)
		if err != nil {
			return fmt.Errorf("wa.New: %w", err)
		}
		defer func() { _ = waCli.Close() }()

		handler := &wa.Handler{
			Store:  s,
			Out:    incoming,
			Notify: w.Invalidate,
			Logger: func(err error) { log.Println("wa.Handler:", err) },
		}
		waCli.AddEventHandler(handler.Adapter(ctx))

		if waCli.NeedsPairing() {
			qrCh, err := waCli.QRChannel(ctx)
			if err != nil {
				return fmt.Errorf("wa.QRChannel: %w", err)
			}
			go renderQRs(qrCh)
		}

		// Connect off the UI goroutine so the window paints immediately;
		// the handshake can take a moment over a flaky connection.
		go func() {
			if err := waCli.Connect(); err != nil {
				log.Println("wa.Connect:", err)
			}
		}()
	} else {
		log.Println("wachat: -no-connect set; running offline against", dbPath)
	}

	// View callbacks translate UI events back into state mutations. The
	// SelectChat / LoadOlder paths do a small keyset read (~1ms over 100k
	// history per our bench) on the UI goroutine — well under one frame.
	// We will move the load off the UI goroutine if measurement ever
	// shows it costs.
	callbacks := ui.ViewCallbacks{
		OnSelectChat: func(jid string) {
			if err := state.SelectChat(ctx, jid); err != nil {
				log.Println("SelectChat:", err)
			}
		},
		OnNearEnd: func() {
			if _, err := state.LoadOlder(ctx); err != nil {
				log.Println("LoadOlder:", err)
			}
		},
	}

	// Theme. material.NewTheme is zero-arg in v0.10; the shaper is set on
	// the returned value.
	theme := material.NewTheme()
	theme.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	var ops op.Ops
	for {
		// Drain any pending events before blocking on the next frame.
		drainIncoming(incoming, state)

		ev := w.Event()
		switch ev := ev.(type) {
		case app.DestroyEvent:
			return ev.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, ev)
			view.Layout(gtx, theme, state, callbacks)
			ev.Frame(gtx.Ops)
		}
	}
}

// drainIncoming folds every pending event into state without blocking.
// Called once per frame; the frame loop then renders from state.
func drainIncoming(in <-chan wa.MessageEvent, st *ui.State) {
	for {
		select {
		case ev := <-in:
			st.OnIncoming(ev)
		default:
			return
		}
	}
}

// sessionDBPath derives the whatsmeow session DB path from the wachat DB
// path: "wachat.db" → "wachat-session.db". The session lives alongside
// the user's message store but in a separate file so either can be wiped
// independently.
func sessionDBPath(dbPath string) string {
	if strings.HasSuffix(dbPath, ".db") {
		return strings.TrimSuffix(dbPath, ".db") + "-session.db"
	}
	return dbPath + "-session"
}

// renderQRs prints incoming QR pairing codes to stdout using qrterminal's
// half-block renderer (twice as dense as the full-block one — fits in a
// typical terminal). The pairing window stays open until whatsmeow
// closes the channel ("success" or "timeout").
func renderQRs(ch <-chan wa.QRItem) {
	for item := range ch {
		switch item.Event {
		case "code":
			fmt.Println()
			fmt.Println("Scan this QR with WhatsApp on your phone (Settings → Linked Devices):")
			qrterminal.GenerateHalfBlock(item.Code, qrterminal.L, os.Stdout)
		case "success":
			fmt.Println("wachat: paired successfully")
			return
		case "timeout":
			fmt.Println("wachat: QR pairing timed out — restart wachat to try again")
			return
		default:
			fmt.Println("wachat: QR pairing event:", item.Event)
		}
	}
}
