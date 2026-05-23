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

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

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
	flag.Parse()

	if *showVersion {
		fmt.Println("wachat", Version)
		return
	}

	go func() {
		if err := run(*dbPath); err != nil {
			log.Println("wachat:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(dbPath string) error {
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

	// Background-goroutine → UI-goroutine handoff channel. Whatsmeow event
	// integration lands in a subsequent commit; the channel + handler are
	// wired now so the frame loop's draining behavior is exercised from
	// day one.
	incoming := make(chan wa.MessageEvent, incomingBuffer)
	_ = wa.Handler{ // referenced by type so the package import is used now
		Store:  s,
		Out:    incoming,
		Notify: w.Invalidate,
	}

	// View callbacks translate UI events back into state mutations. The
	// SelectChat path does a small keyset read (~1ms over 100k history per
	// our bench) on the UI goroutine — well under one frame. We will move
	// the load off the UI goroutine if measurement ever shows it costs.
	callbacks := ui.ViewCallbacks{
		OnSelectChat: func(jid string) {
			if err := state.SelectChat(ctx, jid); err != nil {
				log.Println("SelectChat:", err)
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
