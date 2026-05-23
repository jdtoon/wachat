package ui

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// View bundles Gio widget state — anything the layout code retains
// across frames. Lives alongside State (the reducer) but kept separate so
// the reducer stays Gio-free and easy to unit-test.
type View struct {
	chatList    widget.List
	msgList     widget.List
	chatClicks  []widget.Clickable
	prevNearEnd bool // last frame's "near the end of loaded window" state
}

// NearEndThreshold is the number of rows from the end of the loaded
// message window at which we fire OnNearEnd to keyset-page older
// history. Tuned so a typical scroll velocity has a page ready before
// the user reaches the bottom of the loaded buffer.
const NearEndThreshold = 5

// NewView constructs a View with sensible defaults.
func NewView() *View {
	v := &View{}
	v.chatList.Axis = layout.Vertical
	v.msgList.Axis = layout.Vertical
	return v
}

// ViewCallbacks are the actions Layout can ask the caller to perform.
// Layout itself never calls into the store — store access (and any other
// non-pure work) is the frame loop's job; the View only translates UI
// events into requests.
type ViewCallbacks struct {
	// OnSelectChat fires when the user clicks a chat row.
	OnSelectChat func(jid string)
	// OnNearEnd fires once when the message list scrolls to within
	// NearEndThreshold rows of the end of the loaded buffer — the
	// caller should LoadOlder() in response. Re-fires only after the
	// scroll position leaves the trigger zone and re-enters it (e.g.
	// after a successful page load extends the buffer).
	OnNearEnd func()
}

// Layout renders the two-pane view: chat list on the left (fixed 300dp),
// message bubbles on the right (flexed). Both panes are virtualized — the
// per-row builders are invoked only for items in the visible window
// (CLAUDE.md §6).
//
// Layout reports dimensions and may invoke cb.OnSelectChat if the user
// clicked a row this frame. Colors, sizing, and motion all come from
// th — never reference raw color literals in widget code.
func (v *View) Layout(gtx layout.Context, th *Theme, st *State, cb ViewCallbacks) layout.Dimensions {
	// Keep the clickables slice in sync with the chat slice. Reallocate
	// only when the count changes — Clickable widgets must persist across
	// frames so their internal click state is preserved.
	if len(v.chatClicks) != len(st.Chats) {
		v.chatClicks = make([]widget.Clickable, len(st.Chats))
	}

	// Translate clicks into callbacks BEFORE layout draws — the Clickable
	// records the click during the previous frame's event delivery.
	if cb.OnSelectChat != nil {
		for i := range v.chatClicks {
			if v.chatClicks[i].Clicked(gtx) {
				cb.OnSelectChat(st.Chats[i].JID)
			}
		}
	}

	// Paint the canvas behind everything so material widgets that don't
	// fill their backgrounds (most of them) sit on the right surface.
	paintBackground(gtx, th.Palette.Canvas)

	dims := layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Dp(unit.Dp(300))
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return v.layoutChatList(gtx, th, st)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return verticalDivider(gtx, th.Palette.Divider)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return v.layoutMessages(gtx, th, st)
		}),
	)

	// Pagination trigger: fire on the leading edge of "near end" so we
	// don't spam OnNearEnd every frame the user lingers in the zone. The
	// caller's LoadOlder will extend Messages and the next frame will
	// recompute — false→true transitions only.
	if cb.OnNearEnd != nil && st.SelectedChat != "" {
		nearEnd := isNearEnd(v.msgList.Position, len(st.Messages), NearEndThreshold)
		if nearEnd && !v.prevNearEnd {
			cb.OnNearEnd()
		}
		v.prevNearEnd = nearEnd
	} else {
		v.prevNearEnd = false
	}

	return dims
}

// isNearEnd reports whether the list's last visible row sits within
// threshold rows of the end of the loaded buffer.
//
// Pure function for testability — kept package-private. Position values
// come from widget.List after a layout pass.
func isNearEnd(pos layout.Position, total, threshold int) bool {
	if total == 0 || pos.Count == 0 {
		return false
	}
	lastVisible := pos.First + pos.Count
	return lastVisible >= total-threshold
}

func (v *View) layoutChatList(gtx layout.Context, th *Theme, st *State) layout.Dimensions {
	mat := th.Material()
	return material.List(mat, &v.chatList).Layout(gtx, len(st.Chats), func(gtx layout.Context, i int) layout.Dimensions {
		c := st.Chats[i]
		return v.chatClicks[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top: th.RowPad(), Bottom: th.RowPad(),
				Left: th.Spacing.M, Right: th.Spacing.M,
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				name := material.Label(mat, th.Type.Title, displayName(c))
				name.Color = th.Palette.TextPrimary
				sub := material.Label(mat, th.Type.Meta, chatSubtitle(c))
				sub.Color = th.Palette.TextSecondary
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(name.Layout),
					layout.Rigid(sub.Layout),
				)
			})
		})
	})
}

func (v *View) layoutMessages(gtx layout.Context, th *Theme, st *State) layout.Dimensions {
	mat := th.Material()
	if st.SelectedChat == "" {
		empty := material.Label(mat, th.Type.Body, "Select a chat")
		empty.Color = th.Palette.TextSecondary
		return layout.Center.Layout(gtx, empty.Layout)
	}
	return material.List(mat, &v.msgList).Layout(gtx, len(st.Messages), func(gtx layout.Context, i int) layout.Dimensions {
		m := st.Messages[i]
		return layout.Inset{
			Top: th.Spacing.XS, Bottom: th.Spacing.XS,
			Left: th.Spacing.M, Right: th.Spacing.M,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			body := material.Label(mat, th.Type.Body, m.Body)
			body.Color = th.Palette.TextPrimary
			return body.Layout(gtx)
		})
	})
}

// displayName picks the human-readable label for a chat row. Falls back
// to the JID if we have no name yet — better than a blank row.
func displayName(c ChatSummary) string {
	if c.Name != "" {
		return c.Name
	}
	return c.JID
}

// chatSubtitle is the small line under the chat name. Shows unread count
// when non-zero plus a friendly relative time for last_ts.
func chatSubtitle(c ChatSummary) string {
	when := "—"
	if c.LastTS > 0 {
		when = humanTime(time.UnixMilli(c.LastTS))
	}
	if c.Unread > 0 {
		return fmt.Sprintf("%s · %d unread", when, c.Unread)
	}
	return when
}

// humanTime is a very small relative-time formatter. Avoids a dependency.
func humanTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("2006-01-02")
	}
}

// verticalDivider draws a 1dp-wide line of c between the panes.
func verticalDivider(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	w := gtx.Dp(unit.Dp(1))
	h := gtx.Constraints.Max.Y
	defer clip.Rect(image.Rect(0, 0, w, h)).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// paintBackground fills the current constraints with c. Used to apply
// the canvas color before the panes paint themselves; without this
// material widgets that don't fill their backgrounds inherit black.
func paintBackground(gtx layout.Context, c color.NRGBA) {
	r := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	defer clip.Rect(r).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}
