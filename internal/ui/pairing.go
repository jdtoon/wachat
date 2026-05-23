package ui

import (
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/jdtoon/wachat/internal/wa"
)

// PairingPhase tracks where we are in the device-linking flow.
type PairingPhase int

const (
	// PairingIdle: no pairing flow active (we're already paired).
	PairingIdle PairingPhase = iota
	// PairingWaitingQR: a QR payload is being shown; user should scan
	// from WhatsApp on their phone.
	PairingWaitingQR
	// PairingScanned: phone has scanned; whatsmeow is finalizing the
	// handshake.
	PairingScanned
	// PairingSyncing: paired; history is syncing.
	PairingSyncing
	// PairingReady: ready to use; the caller can switch to the main view.
	PairingReady
	// PairingFailed: terminal failure; caller should show a retry.
	PairingFailed
)

// PairingView renders the device-linking screen. It is a pure layout
// surface — the state machine is driven by the caller forwarding QR
// channel events and connection events via Set methods.
type PairingView struct {
	phase  PairingPhase
	qrCode string
	errMsg string
}

// NewPairingView constructs a view in PairingWaitingQR with no payload.
func NewPairingView() *PairingView {
	return &PairingView{phase: PairingWaitingQR}
}

// Phase reports the current state. Caller switches to the main view
// when this returns PairingReady.
func (p *PairingView) Phase() PairingPhase { return p.phase }

// SetPhase forces the pairing phase. Used by the frame loop to drive
// PairingReady on events.Connected, PairingFailed on errors, etc.
func (p *PairingView) SetPhase(ph PairingPhase) { p.phase = ph }

// SetError moves into PairingFailed and stores a user-facing message.
func (p *PairingView) SetError(msg string) {
	p.phase = PairingFailed
	p.errMsg = msg
}

// HandleQR consumes a wa.QRItem and updates the state machine
// accordingly.
//
// Mapping:
//
//	"code"    → store the payload, stay in WaitingQR
//	"success" → move to Scanned (events.PairSuccess later → Ready)
//	"timeout" → Failed with a timeout message
//	anything else → Failed with the event name
func (p *PairingView) HandleQR(item wa.QRItem) {
	switch item.Event {
	case "code":
		p.qrCode = item.Code
		p.phase = PairingWaitingQR
	case "success":
		p.phase = PairingScanned
	case "timeout":
		p.SetError("QR pairing timed out — relaunch wachat to try again")
	default:
		p.SetError("pairing event: " + item.Event)
	}
}

// Layout renders the pairing screen.
func (p *PairingView) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	mat := th.Material()
	paintBackground(gtx, th.Palette.Canvas)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.Label(mat, th.Type.Display, "Link to WhatsApp")
				title.Color = th.Palette.TextPrimary
				title.Alignment = text.Middle
				return title.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: th.Spacing.L}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutBody(gtx, th)
			}),
			layout.Rigid(layout.Spacer{Height: th.Spacing.L}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutStatus(gtx, th)
			}),
		)
	})
}

func (p *PairingView) layoutBody(gtx layout.Context, th *Theme) layout.Dimensions {
	mat := th.Material()
	switch p.phase {
	case PairingWaitingQR:
		if p.qrCode == "" {
			lbl := material.Label(mat, th.Type.Body, "waiting for QR…")
			lbl.Color = th.Palette.TextSecondary
			return lbl.Layout(gtx)
		}
		return layoutQR(gtx, th, p.qrCode, gtx.Dp(unit.Dp(320)))
	case PairingScanned:
		lbl := material.Label(mat, th.Type.Body, "Phone scanned — finalizing…")
		lbl.Color = th.Palette.TextPrimary
		return lbl.Layout(gtx)
	case PairingSyncing:
		lbl := material.Label(mat, th.Type.Body, "Syncing history…")
		lbl.Color = th.Palette.TextPrimary
		return lbl.Layout(gtx)
	case PairingReady:
		lbl := material.Label(mat, th.Type.Body, "Ready.")
		lbl.Color = th.Palette.Accent
		return lbl.Layout(gtx)
	case PairingFailed:
		lbl := material.Label(mat, th.Type.Body, "Pairing failed")
		lbl.Color = th.Palette.TextPrimary
		return lbl.Layout(gtx)
	default:
		return layout.Dimensions{}
	}
}

func (p *PairingView) layoutStatus(gtx layout.Context, th *Theme) layout.Dimensions {
	mat := th.Material()
	msg := ""
	switch p.phase {
	case PairingWaitingQR:
		msg = "Open WhatsApp → Settings → Linked Devices → Link a Device, then scan the code."
	case PairingScanned:
		msg = "Hold tight — this can take a few seconds."
	case PairingSyncing:
		msg = "Pulling chat history. The window will switch over automatically."
	case PairingFailed:
		msg = p.errMsg
		if msg == "" {
			msg = "Restart wachat to try again."
		}
	}
	if msg == "" {
		return layout.Dimensions{}
	}
	lbl := material.Label(mat, th.Type.Meta, msg)
	lbl.Color = th.Palette.TextSecondary
	lbl.Alignment = text.Middle
	lbl.MaxLines = 3
	return lbl.Layout(gtx)
}
