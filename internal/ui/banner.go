package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget/material"
)

// ConnState is the current network connection state surfaced to the UI.
type ConnState int

const (
	// ConnConnected: client is connected and authenticated.
	ConnConnected ConnState = iota
	// ConnConnecting: handshake in progress (initial or auto-reconnect).
	ConnConnecting
	// ConnDisconnected: websocket dropped; auto-reconnect is retrying.
	ConnDisconnected
	// ConnLoggedOut: device was unlinked by the phone; re-pairing
	// required.
	ConnLoggedOut
)

// LayoutConnectionBanner renders a thin top strip when the connection
// is in a non-OK state. ConnConnected returns zero dimensions so the
// banner doesn't take any space when everything is fine.
func LayoutConnectionBanner(gtx layout.Context, th *Theme, state ConnState) layout.Dimensions {
	msg, fg, bg := bannerCopy(th, state)
	if msg == "" {
		return layout.Dimensions{}
	}
	mat := th.Material()
	return paintFilledStrip(gtx, bg, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: th.Spacing.XS, Bottom: th.Spacing.XS,
			Left: th.Spacing.M, Right: th.Spacing.M,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(mat, th.Type.Meta, msg)
			lbl.Color = fg
			lbl.Alignment = text.Middle
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		})
	})
}

// bannerCopy returns the visible string + foreground/background for a
// given ConnState. Empty msg means "don't draw the banner."
func bannerCopy(th *Theme, state ConnState) (msg string, fg, bg color.NRGBA) {
	switch state {
	case ConnConnecting:
		return "Connecting…", th.Palette.AccentText, th.Palette.Accent
	case ConnDisconnected:
		return "Offline — reconnecting…", th.Palette.AccentText, dim(th.Palette.Accent)
	case ConnLoggedOut:
		return "Signed out from phone — relaunch to re-pair", th.Palette.AccentText, th.Palette.Unread
	}
	return "", th.Palette.TextPrimary, th.Palette.Surface
}

// paintFilledStrip fills the laid-out content's vertical extent with bg.
func paintFilledStrip(gtx layout.Context, bg color.NRGBA, w layout.Widget) layout.Dimensions {
	// Record content first, then paint behind it (same trick as
	// roundedFill / paintHeaderSurface).
	macro := op.Record(gtx.Ops)
	dims := w(gtx)
	call := macro.Stop()
	rect := image.Rect(0, 0, gtx.Constraints.Max.X, dims.Size.Y)
	defer clip.Rect(rect).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: bg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, dims.Size.Y)}
}
