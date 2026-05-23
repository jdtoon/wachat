package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Composer is the message-input widget at the bottom of the message
// pane. Holds the Editor's text and the send button's click state
// across frames.
//
// Behavior:
//   - Enter sends; Shift+Enter inserts a newline (a convention every
//     chat user expects).
//   - The send button is enabled only when the editor has non-empty,
//     non-whitespace content.
//   - On send, the editor is cleared and ready for the next message.
//
// The composer is layout-only — it never calls the store or the
// network. Layout returns a "submit requested" signal via OnSend.
type Composer struct {
	editor widget.Editor
	send   widget.Clickable
}

// NewComposer constructs a Composer in its initial state.
func NewComposer() *Composer {
	c := &Composer{}
	c.editor.SingleLine = false
	c.editor.Submit = false // we handle Enter manually so Shift+Enter works
	return c
}

// Text returns the current editor contents.
func (c *Composer) Text() string { return c.editor.Text() }

// SetText replaces the editor contents (e.g. when restoring a draft).
func (c *Composer) SetText(s string) {
	c.editor.SetText(s)
}

// Clear empties the editor — call after a successful send.
func (c *Composer) Clear() { c.editor.SetText("") }

// Layout renders the composer. Returns the laid-out dimensions; if a
// send was requested this frame (button click OR plain Enter), calls
// onSend with the trimmed body and Clear()s the editor.
func (c *Composer) Layout(gtx layout.Context, th *Theme, onSend func(body string)) layout.Dimensions {
	mat := th.Material()

	// Plain Enter sends; Shift+Enter inserts a newline. We detect this
	// by scanning editor key events. Without this, Editor.Submit would
	// fire on Enter but also intercept Shift+Enter (we want a newline).
	c.handleEnter(gtx, onSend)

	// Send button click path (always present, redundant with Enter for
	// mouse-only users).
	if c.send.Clicked(gtx) && onSend != nil {
		c.submit(onSend)
	}

	return paintComposerSurface(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: th.Spacing.S, Bottom: th.Spacing.S,
			Left: th.Spacing.M, Right: th.Spacing.M,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
				// Multiline editor, flexed to take remaining width.
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(mat, &c.editor, "Type a message")
					ed.Color = th.Palette.TextPrimary
					ed.HintColor = th.Palette.TextSecondary
					return ed.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: th.Spacing.M}.Layout),
				// Send button: rounded accent pill with a single arrow glyph.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return c.send.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layoutSendButton(gtx, th, c.canSend())
					})
				}),
			)
		})
	})
}

// canSend reports whether the current editor content is non-empty
// after trimming whitespace.
func (c *Composer) canSend() bool {
	return strings.TrimSpace(c.editor.Text()) != ""
}

// submit is the shared finalization path: trim, fire callback, clear.
// No-op when content is empty.
func (c *Composer) submit(onSend func(body string)) {
	body := strings.TrimSpace(c.editor.Text())
	if body == "" {
		return
	}
	onSend(body)
	c.Clear()
}

// handleEnter intercepts Enter (without Shift) on the editor and
// triggers submit. Shift+Enter falls through to the editor so it
// inserts a newline.
//
// Implementation note: we route a key.Filter through the editor and
// check the modifiers. If Editor.Submit was set true, Enter ALWAYS
// submits and Shift+Enter is impossible — so we set Submit=false and
// do the dispatch here.
func (c *Composer) handleEnter(gtx layout.Context, onSend func(body string)) {
	if onSend == nil {
		return
	}
	for {
		ev, ok := gtx.Event(key.Filter{
			Focus:    &c.editor,
			Name:     key.NameReturn,
			Optional: key.ModShift,
		})
		if !ok {
			break
		}
		ke, isKey := ev.(key.Event)
		if !isKey || ke.State != key.Press {
			continue
		}
		if ke.Modifiers.Contain(key.ModShift) {
			// Shift+Enter — let the editor handle it (newline).
			c.editor.Insert("\n")
			continue
		}
		c.submit(onSend)
	}
}

// layoutSendButton draws the accent-colored send button. Dimmed when
// the editor is empty so the user gets a visual "nothing to send".
func layoutSendButton(gtx layout.Context, th *Theme, enabled bool) layout.Dimensions {
	mat := th.Material()
	bg := th.Palette.Accent
	if !enabled {
		bg = dim(bg)
	}
	lbl := material.Label(mat, th.Type.Label, "Send")
	lbl.Color = th.Palette.AccentText
	lbl.Alignment = text.Middle
	return roundedFill(gtx, bg, th.Radius.Button, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: th.Spacing.S, Bottom: th.Spacing.S,
			Left: th.Spacing.M, Right: th.Spacing.M,
		}.Layout(gtx, lbl.Layout)
	})
}

// paintComposerSurface fills the composer's background with
// th.Palette.Surface so the input area visually separates from the
// canvas behind the message scroll.
func paintComposerSurface(gtx layout.Context, th *Theme, w layout.Widget) layout.Dimensions {
	// Top divider line (1dp) to clearly separate from messages above.
	dividerH := gtx.Dp(unit.Dp(1))

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = dividerH
			gtx.Constraints.Max.Y = dividerH
			r := image.Rect(0, 0, gtx.Constraints.Max.X, dividerH)
			defer clip.Rect(r).Push(gtx.Ops).Pop()
			paint.ColorOp{Color: th.Palette.Divider}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, dividerH)}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Background fill behind the inset content.
			macro := op.Record(gtx.Ops)
			dims := w(gtx)
			call := macro.Stop()
			rect := image.Rect(0, 0, gtx.Constraints.Max.X, dims.Size.Y)
			defer clip.Rect(rect).Push(gtx.Ops).Pop()
			paint.ColorOp{Color: th.Palette.Surface}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			call.Add(gtx.Ops)
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, dims.Size.Y)}
		}),
	)
}

// dim returns c with alpha halved — used for disabled controls.
func dim(c color.NRGBA) color.NRGBA {
	c.A /= 2
	return c
}
