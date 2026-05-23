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
func layoutBubble(gtx layout.Context, th *Theme, m store.Message, group GroupPosition, fromMe bool) layout.Dimensions {
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
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(body.Layout),
						layout.Rigid(layout.Spacer{Height: th.Spacing.XXS}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutBubbleMeta(gtx, th, m, fromMe)
						}),
					)
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
