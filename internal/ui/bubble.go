package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/jdtoon/wachat/internal/store"
)

// layoutBubble renders a single message bubble. Sent (from us) bubbles
// align right and use Palette.BubbleSent; received bubbles align left
// and use Palette.BubbleRecv. Position in a sender group controls the
// vertical spacing — middle and head bubbles tighten toward the bubble
// below them.
//
// fromMe is the caller's classification of "this message was sent by
// the local user." Currently derived from sender JID being empty
// (matches cmd/seed and the wa.Handler convention). When v0.0.4 lands
// real outbound messages we'll set this off `store.Message.SenderJID`
// against the device's own JID.
//
// senderLabel, if non-empty, is rendered above the bubble for the
// head of a sender group (or solo bubble) — used by group chats to
// distinguish participants. Pass "" for 1:1 chats.
func layoutBubble(gtx layout.Context, th *Theme, m store.Message, group GroupPosition, fromMe bool, senderLabel string) layout.Dimensions {
	mat := th.Material()
	bg := th.Palette.BubbleRecv
	align := layout.W
	if fromMe {
		bg = th.Palette.BubbleSent
		align = layout.E
	}

	// Vertical spacing between consecutive bubbles in the same group is
	// tight; across groups it opens up.
	topPad := th.Spacing.S
	if group == GroupMiddle || group == GroupTail {
		topPad = th.Spacing.XXS
	}
	bottomPad := th.Spacing.XXS
	if group == GroupTail || group == GroupSolo {
		bottomPad = th.Spacing.S
	}

	return layout.Inset{
		Top: topPad, Bottom: bottomPad,
		Left: th.Spacing.M, Right: th.Spacing.M,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return align.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// Cap bubble width at ~70% of the message pane so long
			// lines wrap naturally and don't span the entire view.
			max := gtx.Constraints.Max.X * 7 / 10
			if max < gtx.Dp(unit.Dp(80)) {
				max = gtx.Dp(unit.Dp(80))
			}
			gtx.Constraints.Max.X = max
			return roundedFill(gtx, bg, th.Radius.Bubble, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top: th.Spacing.S, Bottom: th.Spacing.S,
					Left: th.Spacing.M, Right: th.Spacing.M,
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					body := material.Label(mat, th.Type.Body, m.Body)
					body.Color = th.Palette.TextPrimary

					children := []layout.FlexChild{}
					// Sender label only on the FIRST bubble of a sender
					// run (Head or Solo) — middle/tail share the same
					// sender and don't need it repeated.
					if senderLabel != "" && (group == GroupHead || group == GroupSolo) {
						lbl := material.Label(mat, th.Type.Label, senderLabel)
						lbl.Color = senderLabelColor(senderLabel, th)
						lbl.MaxLines = 1
						children = append(children,
							layout.Rigid(lbl.Layout),
							layout.Rigid(layout.Spacer{Height: th.Spacing.XXS}.Layout),
						)
					}
					children = append(children,
						layout.Rigid(body.Layout),
						layout.Rigid(layout.Spacer{Height: th.Spacing.XXS}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutBubbleMeta(gtx, th, m, fromMe)
						}),
					)
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			})
		})
	})
}

// roundedFill paints bg into a rounded rectangle and lays w on top.
// The rectangle takes the size of the content (w returns it).
func roundedFill(gtx layout.Context, bg color.NRGBA, radius unit.Dp, w layout.Widget) layout.Dimensions {
	// Record the content to a macro so we know its size, then paint the
	// rounded fill behind it.
	macro := op.Record(gtx.Ops)
	dims := w(gtx)
	call := macro.Stop()

	rr := gtx.Dp(radius)
	rect := image.Rect(0, 0, dims.Size.X, dims.Size.Y)
	defer clip.UniformRRect(rect, rr).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: bg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	call.Add(gtx.Ops)
	return dims
}

// formatBubbleMeta returns the small line shown at the bottom of a
// bubble — currently just "HH:MM". The receipt tick is rendered
// separately so it can be colored (blue ticks for "read").
func formatBubbleMeta(m store.Message) string {
	if m.TS == 0 {
		return ""
	}
	return time.UnixMilli(m.TS).Format("15:04")
}

// layoutBubbleMeta draws the bottom row of a bubble: the time on the
// left, and (for outgoing messages only) a delivery tick on the
// right. Tick glyph + color follow WhatsApp convention:
//
//	pending   → ⏱  (clock; not yet ack'd by server)
//	sent      → ✓
//	delivered → ✓✓
//	read      → ✓✓ in accent color (the "blue ticks")
//	played    → ✓✓ in accent color (voice notes; future)
func layoutBubbleMeta(gtx layout.Context, th *Theme, m store.Message, fromMe bool) layout.Dimensions {
	mat := th.Material()
	timeLbl := material.Label(mat, th.Type.Meta, formatBubbleMeta(m))
	timeLbl.Color = th.Palette.TextSecondary

	if !fromMe {
		return timeLbl.Layout(gtx)
	}

	glyph, useAccent := receiptGlyph(m.Status)
	if glyph == "" {
		return timeLbl.Layout(gtx)
	}
	tick := material.Label(mat, th.Type.Meta, glyph)
	if useAccent {
		tick.Color = th.Palette.Accent
	} else {
		tick.Color = th.Palette.TextSecondary
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(timeLbl.Layout),
		layout.Rigid(layout.Spacer{Width: th.Spacing.XS}.Layout),
		layout.Rigid(tick.Layout),
	)
}

// senderLabelColor picks a deterministic hue per sender name so each
// participant in a group is visually distinct (cheap, no images). We
// reuse the same hash strategy as avatar tints so a person's bubble
// label and chat-row avatar share a color family.
func senderLabelColor(name string, th *Theme) color.NRGBA {
	// Hash the name to a hue and saturate it more strongly than
	// avatars (which sit behind text and need to stay subtle).
	const offset = 14695981039346656037
	const prime = 1099511628211
	h := uint64(offset)
	for i := 0; i < len(name); i++ {
		h ^= uint64(name[i])
		h *= prime
	}
	hue := float64(h%360) / 360.0
	return hsvToRGB(hue, 0.55, 0.50)
}

// receiptGlyph returns the tick glyph + whether the accent color
// should be used. Pure function for unit testing.
func receiptGlyph(status string) (glyph string, accent bool) {
	switch status {
	case store.StatusPending:
		return "⏱", false
	case store.StatusSent:
		return "✓", false
	case store.StatusDelivered:
		return "✓✓", false
	case store.StatusRead, store.StatusPlayed:
		return "✓✓", true
	}
	return "", false
}
