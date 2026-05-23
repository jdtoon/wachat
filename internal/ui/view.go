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
	chatList   widget.List
	msgList    widget.List
	chatClicks []widget.Clickable
}

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
	OnSelectChat func(jid string)
}

// Layout renders the two-pane view: chat list on the left (fixed 300dp),
// message bubbles on the right (flexed). Both panes are virtualized — the
// per-row builders are invoked only for items in the visible window
// (CLAUDE.md §6).
//
// Layout reports dimensions and may invoke cb.OnSelectChat if the user
// clicked a row this frame.
func (v *View) Layout(gtx layout.Context, th *material.Theme, st *State, cb ViewCallbacks) layout.Dimensions {
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

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Dp(unit.Dp(300))
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return v.layoutChatList(gtx, th, st)
		}),
		layout.Rigid(verticalDivider),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return v.layoutMessages(gtx, th, st)
		}),
	)
}

func (v *View) layoutChatList(gtx layout.Context, th *material.Theme, st *State) layout.Dimensions {
	return material.List(th, &v.chatList).Layout(gtx, len(st.Chats), func(gtx layout.Context, i int) layout.Dimensions {
		c := st.Chats[i]
		return v.chatClicks[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top: unit.Dp(8), Bottom: unit.Dp(8),
				Left: unit.Dp(12), Right: unit.Dp(12),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Body1(th, displayName(c)).Layout),
					layout.Rigid(material.Caption(th, chatSubtitle(c)).Layout),
				)
			})
		})
	})
}

func (v *View) layoutMessages(gtx layout.Context, th *material.Theme, st *State) layout.Dimensions {
	if st.SelectedChat == "" {
		return layout.Center.Layout(gtx, material.Body1(th, "Select a chat").Layout)
	}
	return material.List(th, &v.msgList).Layout(gtx, len(st.Messages), func(gtx layout.Context, i int) layout.Dimensions {
		m := st.Messages[i]
		return layout.Inset{
			Top: unit.Dp(4), Bottom: unit.Dp(4),
			Left: unit.Dp(12), Right: unit.Dp(12),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return material.Body1(th, m.Body).Layout(gtx)
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

// verticalDivider draws a 1dp-wide grey line between the panes.
func verticalDivider(gtx layout.Context) layout.Dimensions {
	w := gtx.Dp(unit.Dp(1))
	h := gtx.Constraints.Max.Y
	defer clip.Rect(image.Rect(0, 0, w, h)).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: color.NRGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(w, h)}
}
