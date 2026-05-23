package ui

import (
	"image"
	"image/color"
	"sort"
	"time"

	"gioui.org/font"
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
//
// reactions is the set of reactions on this message; pass nil for
// none. The bubble renders a chip cluster below the meta row.
//
// thumbnail is an already-decoded image for media messages; pass nil
// for text-only or when the decode hasn't happened yet (the cache +
// tracker plumb this in over a frame).
func layoutBubble(gtx layout.Context, th *Theme, m store.Message, group GroupPosition, fromMe bool, senderLabel string, reactions []store.Reaction, thumbnail image.Image) layout.Dimensions {
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

					// Quoted-message block (if this is a reply).
					if m.QuotedBody != "" || m.QuotedWAID != "" {
						children = append(children,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutQuotedBlock(gtx, th, m, fromMe)
							}),
							layout.Rigid(layout.Spacer{Height: th.Spacing.XS}.Layout),
						)
					}

					// Media thumbnail or media-type pill ABOVE the body
					// caption.
					if m.MediaType != "" {
						children = append(children,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutMediaBlock(gtx, th, m, thumbnail)
							}),
							layout.Rigid(layout.Spacer{Height: th.Spacing.XS}.Layout),
						)
					}

					// Body — revoked messages show a placeholder.
					bodyText, italicized := bubbleBodyText(m)
					if bodyText == "" && m.MediaType != "" {
						// No caption — skip the body Label entirely.
					}
					body := material.Label(mat, th.Type.Body, bodyText)
					body.Color = th.Palette.TextPrimary
					if italicized {
						body.Color = th.Palette.TextSecondary
						body.Font.Style = font.Italic
					}
					if bodyText != "" {
						children = append(children, layout.Rigid(body.Layout))
					} else {
						_ = body // silence unused
					}
					children = append(children,
						layout.Rigid(layout.Spacer{Height: th.Spacing.XXS}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutBubbleMeta(gtx, th, m, fromMe)
						}),
					)
					_ = mat // silence (already used above)
					if len(reactions) > 0 {
						children = append(children,
							layout.Rigid(layout.Spacer{Height: th.Spacing.XS}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutReactions(gtx, th, reactions)
							}),
						)
					}
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

// layoutMediaBlock renders the media portion of a bubble: a thumbnail
// when one is available, otherwise a small media-type pill so the
// user still knows an attachment was sent.
func layoutMediaBlock(gtx layout.Context, th *Theme, m store.Message, thumbnail image.Image) layout.Dimensions {
	if thumbnail != nil {
		return layoutThumbnail(gtx, th, thumbnail, unit.Dp(220))
	}
	mat := th.Material()
	glyph, label := mediaTypeGlyph(m.MediaType)
	lbl := material.Label(mat, th.Type.Label, glyph+" "+label)
	lbl.Color = th.Palette.TextSecondary
	return roundedFill(gtx, th.Palette.Surface, th.Radius.Button, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: th.Spacing.XS, Bottom: th.Spacing.XS,
			Left: th.Spacing.S, Right: th.Spacing.S,
		}.Layout(gtx, lbl.Layout)
	})
}

// mediaTypeGlyph returns the glyph + label for a media-type pill.
// Pure helper for unit testing.
func mediaTypeGlyph(mediaType string) (glyph, label string) {
	switch mediaType {
	case "image":
		return "📷", "Photo"
	case "video":
		return "🎬", "Video"
	case "audio":
		return "🎙", "Voice note"
	case "document":
		return "📄", "Document"
	case "sticker":
		return "🎴", "Sticker"
	}
	return "📎", "Attachment"
}

// bubbleBodyText returns what to render as the bubble's main body.
// Revoked messages return a placeholder + italic flag. Edited messages
// append a small "(edited)" marker but stay plain. Pure for tests.
func bubbleBodyText(m store.Message) (text string, italic bool) {
	if m.Revoked {
		return "🚫 message deleted", true
	}
	if m.Edited {
		if m.Body == "" {
			return "(edited)", true
		}
		return m.Body + "  (edited)", false
	}
	return m.Body, false
}

// layoutQuotedBlock renders the small accent-bordered block above the
// reply's body showing the quoted snippet. Per design.md §3 — a thin
// accent-bordered block above the text showing the quoted snippet;
// tap to jump to the original (jump-to lands in a follow-up).
func layoutQuotedBlock(gtx layout.Context, th *Theme, m store.Message, fromMe bool) layout.Dimensions {
	mat := th.Material()
	// Background: a slightly darker shade than the bubble itself.
	bg := tintQuoteBackground(th, fromMe)
	return roundedFill(gtx, bg, th.Radius.Button, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: th.Spacing.XS, Bottom: th.Spacing.XS,
			Left: th.Spacing.S, Right: th.Spacing.S,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			senderText := m.QuotedSender
			if senderText == "" {
				senderText = "Quoted"
			}
			senderLbl := material.Label(mat, th.Type.Meta, senderText)
			senderLbl.Color = th.Palette.Accent
			senderLbl.MaxLines = 1
			bodyLbl := material.Label(mat, th.Type.Meta, m.QuotedBody)
			bodyLbl.Color = th.Palette.TextSecondary
			bodyLbl.MaxLines = 2
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(senderLbl.Layout),
				layout.Rigid(bodyLbl.Layout),
			)
		})
	})
}

// tintQuoteBackground returns a slightly-darker shade of the bubble
// fill so the quoted block visually nests inside it. We just dim the
// bubble color a touch.
func tintQuoteBackground(th *Theme, fromMe bool) color.NRGBA {
	bg := th.Palette.BubbleRecv
	if fromMe {
		bg = th.Palette.BubbleSent
	}
	return color.NRGBA{
		R: scaleChan(bg.R, 0.88),
		G: scaleChan(bg.G, 0.88),
		B: scaleChan(bg.B, 0.88),
		A: bg.A,
	}
}

func scaleChan(v uint8, s float64) uint8 {
	r := float64(v) * s
	if r < 0 {
		return 0
	}
	if r > 255 {
		return 255
	}
	return uint8(r)
}

// summarizeReactions groups a slice of reactions by emoji, returning
// (emoji, count) pairs in count-desc order. Pure for testing.
func summarizeReactions(rs []store.Reaction) []ReactionTally {
	if len(rs) == 0 {
		return nil
	}
	counts := make(map[string]int, len(rs))
	for _, r := range rs {
		if r.Emoji == "" {
			continue
		}
		counts[r.Emoji]++
	}
	if len(counts) == 0 {
		return nil
	}
	out := make([]ReactionTally, 0, len(counts))
	for e, c := range counts {
		out = append(out, ReactionTally{Emoji: e, Count: c})
	}
	// Stable sort: count desc, then emoji asc.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Emoji < out[j].Emoji
	})
	return out
}

// ReactionTally is one row of the reaction summary cluster (emoji
// glyph + how many people reacted with it).
type ReactionTally struct {
	Emoji string
	Count int
}

// layoutReactions renders the chip cluster: one chip per unique
// emoji, "👍 3" style. Chips wrap onto multiple lines if the bubble
// is narrow.
func layoutReactions(gtx layout.Context, th *Theme, rs []store.Reaction) layout.Dimensions {
	tallies := summarizeReactions(rs)
	if len(tallies) == 0 {
		return layout.Dimensions{}
	}
	mat := th.Material()
	children := make([]layout.FlexChild, 0, len(tallies))
	for _, t := range tallies {
		t := t
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: th.Spacing.XS}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				txt := t.Emoji
				if t.Count > 1 {
					txt = t.Emoji + " " + reactionCountText(t.Count)
				}
				lbl := material.Label(mat, th.Type.Meta, txt)
				lbl.Color = th.Palette.TextPrimary
				return roundedFill(gtx, th.Palette.Surface, th.Radius.Button, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top: th.Spacing.XXS, Bottom: th.Spacing.XXS,
						Left: th.Spacing.XS, Right: th.Spacing.XS,
					}.Layout(gtx, lbl.Layout)
				})
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

// reactionCountText renders the count number — formatCount style but
// without the cap (reactions can legitimately go above 99 on busy
// chats; we don't fuss with that).
func reactionCountText(n int) string {
	if n <= 0 {
		return "0"
	}
	digits := [4]byte{}
	i := len(digits)
	for n > 0 && i > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
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
