package ui

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/jdtoon/wachat/internal/media"
	"github.com/jdtoon/wachat/internal/store"
)

// View bundles Gio widget state — anything the layout code retains
// across frames. Lives alongside State (the reducer) but kept separate so
// the reducer stays Gio-free and easy to unit-test.
type View struct {
	chatList    widget.List
	msgList     widget.List
	chatClicks  []widget.Clickable
	composer    *Composer
	search      *SearchBar
	themeBtn    widget.Clickable // header theme toggle
	densityBtn  widget.Clickable // header density toggle
	backBtn     widget.Clickable // narrow-window back button
	prevNearEnd bool             // last frame's "near the end of loaded window" state

	// Thumbnail decode-on-visible plumbing. The cache holds decoded
	// images; the tracker computes Decode/Release deltas as the
	// visible message window changes. visiblePaths is scratch space
	// rebuilt each frame.
	imgCache     *media.Cache
	imgTracker   *media.Tracker
	visiblePaths []string
}

// NarrowWindowDp is the width below which we collapse the two-pane
// layout into a single pane (sidebar OR conversation, with a back
// button). From docs/design.md §2.
const NarrowWindowDp = 760

// NearEndThreshold is the number of rows from the end of the loaded
// message window at which we fire OnNearEnd to keyset-page older
// history. Tuned so a typical scroll velocity has a page ready before
// the user reaches the bottom of the loaded buffer.
const NearEndThreshold = 5

// NewView constructs a View with sensible defaults.
func NewView() *View {
	v := &View{composer: NewComposer(), search: NewSearchBar()}
	v.chatList.Axis = layout.Vertical
	v.msgList.Axis = layout.Vertical
	// Messages are rendered newest-at-bottom; default to anchoring the
	// view to the end so new arrivals are immediately visible.
	v.msgList.ScrollToEnd = true
	v.imgCache = NewThumbnailCache()
	v.imgTracker = media.NewTracker(v.imgCache)
	return v
}

// thumbnailFor returns the decoded thumbnail for the given on-disk
// path. Returns nil if the path is empty or decoding has not yet
// happened. Used by the bubble code; the tracker's SetVisible
// invocation later in the frame brings new paths into the cache.
func (v *View) thumbnailFor(path string) image.Image {
	if v.imgCache == nil || path == "" {
		return nil
	}
	img, err := v.imgCache.Decode(path)
	if err != nil {
		return nil
	}
	// Decode increments refcount; release once so the count balances
	// with the SetVisible Decode at end-of-frame.
	v.imgCache.Release(path)
	return img
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
	// OnSend fires when the user submits the composer (Enter or Send
	// button). body is the trimmed text. Caller wires this to
	// wa.SendText + state.AddOptimistic.
	OnSend func(chatJID, body string)
	// OnSearch fires when the user submits a query in the search bar.
	// Empty query means "clear results."
	OnSearch func(query string)
	// OnJumpToMessage fires when the user clicks a search hit. Caller
	// resolves it via state.JumpToMessage.
	OnJumpToMessage func(hit store.SearchHit)
	// OnToggleTheme fires when the user clicks the theme toggle in the
	// header. Caller swaps the live Theme and persists the choice.
	OnToggleTheme func()
	// OnToggleDensity fires for the density toggle (compact /
	// comfortable). Caller flips Theme.Density and persists.
	OnToggleDensity func()
	// OnBack fires from the narrow-window single-pane back arrow.
	OnBack func()
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

	// Header-button clicks → callbacks. Always evaluated; the buttons
	// are rendered inside the header which may sit on either pane in
	// the narrow layout.
	if v.themeBtn.Clicked(gtx) && cb.OnToggleTheme != nil {
		cb.OnToggleTheme()
	}
	if v.densityBtn.Clicked(gtx) && cb.OnToggleDensity != nil {
		cb.OnToggleDensity()
	}
	if v.backBtn.Clicked(gtx) && cb.OnBack != nil {
		cb.OnBack()
	}

	// Paint the canvas behind everything so material widgets that don't
	// fill their backgrounds (most of them) sit on the right surface.
	paintBackground(gtx, th.Palette.Canvas)

	// Narrow-window single-pane mode: show whichever side the user is
	// currently focused on. Toggle via the back arrow in the
	// conversation header.
	winWidthDp := unit.Dp(gtx.Metric.PxToDp(gtx.Constraints.Max.X))
	if winWidthDp < NarrowWindowDp {
		return v.layoutNarrow(gtx, th, st, cb)
	}

	dims := layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Dp(unit.Dp(300))
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return v.layoutSidebar(gtx, th, st, cb)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return verticalDivider(gtx, th.Palette.Divider)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return v.layoutConversation(gtx, th, st, cb)
		}),
	)

	// Pagination trigger: fire on the leading edge of "the user has
	// scrolled near the oldest loaded message".
	if st.SelectedChat != "" {
		v.checkPagingTrigger(v.msgList.Position, len(st.Messages), cb)
	} else {
		v.prevNearEnd = false
	}

	return dims
}

// checkPagingTrigger evaluates the near-oldest-loaded predicate and
// fires cb.OnNearEnd on the leading edge (false→true) so the caller
// doesn't see one fire per frame while the user lingers in the zone.
// Pulled out as a method so it can be unit-tested against synthetic
// layout.Position values without driving the full View.Layout pass.
func (v *View) checkPagingTrigger(pos layout.Position, total int, cb ViewCallbacks) {
	if cb.OnNearEnd == nil {
		v.prevNearEnd = false
		return
	}
	near := isNearOldestLoaded(pos, total, NearEndThreshold)
	if near && !v.prevNearEnd {
		cb.OnNearEnd()
	}
	v.prevNearEnd = near
}

// isNearOldestLoaded reports whether the visible window is within
// threshold rows of the start of the loaded buffer. Because messages
// render newest-at-bottom, the start of the layout is the oldest
// loaded message — scrolling up toward i=0 means "I want more history."
//
// Pure function for testability — kept package-private. Position values
// come from widget.List after a layout pass.
func isNearOldestLoaded(pos layout.Position, total, threshold int) bool {
	if total == 0 || pos.Count == 0 {
		return false
	}
	return pos.First <= threshold
}

// layoutNarrow draws either the sidebar OR the conversation, never
// both. When no chat is selected we show the sidebar (so the user can
// pick one); once selected we show the conversation with a back arrow
// in the header. The user's back-arrow click clears SelectedChat via
// the OnBack callback.
func (v *View) layoutNarrow(gtx layout.Context, th *Theme, st *State, cb ViewCallbacks) layout.Dimensions {
	if st.SelectedChat == "" {
		return v.layoutSidebar(gtx, th, st, cb)
	}
	return v.layoutConversation(gtx, th, st, cb)
}

// layoutSidebar = search input on top, chat list (or search results
// overlay if a search is active) below.
func (v *View) layoutSidebar(gtx layout.Context, th *Theme, st *State, cb ViewCallbacks) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.search.LayoutInput(gtx, th, cb.OnSearch)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if st.Results != nil {
				return v.search.LayoutResults(gtx, th, st, cb.OnJumpToMessage)
			}
			return v.layoutChatList(gtx, th, st)
		}),
	)
}

func (v *View) layoutChatList(gtx layout.Context, th *Theme, st *State) layout.Dimensions {
	mat := th.Material()
	return material.List(mat, &v.chatList).Layout(gtx, len(st.Chats), func(gtx layout.Context, i int) layout.Dimensions {
		c := st.Chats[i]
		return v.chatClicks[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layoutChatRow(gtx, th, c)
		})
	})
}

// layoutConversation is the right-side pane: an optional header above
// the messages list above the composer. Vertical flex.
func (v *View) layoutConversation(gtx layout.Context, th *Theme, st *State, cb ViewCallbacks) layout.Dimensions {
	if st.SelectedChat == "" {
		mat := th.Material()
		empty := material.Label(mat, th.Type.Body, "Select a chat")
		empty.Color = th.Palette.TextSecondary
		return layout.Center.Layout(gtx, empty.Layout)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutHeader(gtx, th, st)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return v.layoutMessages(gtx, th, st)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutTypingIndicator(gtx, th, st)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.composer.Layout(gtx, th, func(body string) {
				if cb.OnSend != nil {
					cb.OnSend(st.SelectedChat, body)
				}
			})
		}),
	)
}

// layoutTypingIndicator shows the "Alice is typing…" line just above
// the composer when there's a live typing event. Renders nothing
// when nobody is composing.
func (v *View) layoutTypingIndicator(gtx layout.Context, th *Theme, st *State) layout.Dimensions {
	nowMS := time.Now().UnixMilli()
	typers := st.ActiveTypers(st.SelectedChat, nowMS)
	if len(typers) == 0 {
		return layout.Dimensions{}
	}
	mat := th.Material()
	// Build a "Alice, Bob is typing…" string. Cap at 3 senders so the
	// line never blows up the layout.
	names := make([]string, 0, 3)
	for i, e := range typers {
		if i >= 3 {
			names = append(names, "…")
			break
		}
		name := st.NameFor(e.SenderJID)
		if name == "" {
			name = e.SenderJID
		}
		names = append(names, name)
	}
	verb := "is typing…"
	if len(typers) > 1 {
		verb = "are typing…"
	}
	msg := joinNames(names) + " " + verb
	return layout.Inset{
		Top: th.Spacing.XS, Bottom: th.Spacing.XXS,
		Left: th.Spacing.M, Right: th.Spacing.M,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Label(mat, th.Type.Meta, msg)
		lbl.Color = th.Palette.TextSecondary
		return lbl.Layout(gtx)
	})
}

// joinNames is a tiny strings.Join replacement so we don't pull
// strings.Join into the layout package just for this.
func joinNames(s []string) string {
	switch len(s) {
	case 0:
		return ""
	case 1:
		return s[0]
	case 2:
		return s[0] + " and " + s[1]
	}
	out := s[0]
	for i := 1; i < len(s)-1; i++ {
		out += ", " + s[i]
	}
	out += ", and " + s[len(s)-1]
	return out
}

// layoutHeader is the conversation header bar: optional back arrow
// (narrow mode), chat name, density toggle, theme toggle. Stays compact.
func (v *View) layoutHeader(gtx layout.Context, th *Theme, st *State) layout.Dimensions {
	mat := th.Material()
	name := headerName(st)
	winWidthDp := unit.Dp(gtx.Metric.PxToDp(gtx.Constraints.Max.X))
	showBack := winWidthDp < NarrowWindowDp

	return paintHeaderSurface(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: th.Spacing.S, Bottom: th.Spacing.S,
			Left: th.Spacing.M, Right: th.Spacing.M,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{}
			if showBack {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return v.backBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(mat, th.Type.Label, "←")
						lbl.Color = th.Palette.TextSecondary
						return layout.Inset{Right: th.Spacing.S}.Layout(gtx, lbl.Layout)
					})
				}))
			}
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(mat, th.Type.Title, name)
				lbl.Color = th.Palette.TextPrimary
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}))
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return v.densityBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layoutHeaderIcon(gtx, th, densityGlyph(th.Density))
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return v.themeBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layoutHeaderIcon(gtx, th, themeGlyph(th.Palette))
					})
				}),
			)
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

// layoutHeaderIcon is a tiny labeled button for the header toggles.
// Uses single glyphs so the row stays compact and language-neutral.
func layoutHeaderIcon(gtx layout.Context, th *Theme, glyph string) layout.Dimensions {
	mat := th.Material()
	lbl := material.Label(mat, th.Type.Label, glyph)
	lbl.Color = th.Palette.TextSecondary
	return layout.Inset{Left: th.Spacing.S, Right: th.Spacing.S}.Layout(gtx, lbl.Layout)
}

func themeGlyph(p Palette) string {
	if p.Canvas == DarkPalette.Canvas {
		return "☀" // currently dark, button switches to light
	}
	return "☾" // currently light, button switches to dark
}

func densityGlyph(d Density) string {
	if d == DensityCompact {
		return "≡" // currently compact
	}
	return "☰" // currently comfortable
}

func paintHeaderSurface(gtx layout.Context, th *Theme, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := w(gtx)
	call := macro.Stop()

	// Surface fill + bottom 1dp divider.
	rect := image.Rect(0, 0, gtx.Constraints.Max.X, dims.Size.Y)
	defer clip.Rect(rect).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: th.Palette.Surface}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	call.Add(gtx.Ops)

	dividerH := gtx.Dp(unit.Dp(1))
	dr := image.Rect(0, dims.Size.Y-dividerH, gtx.Constraints.Max.X, dims.Size.Y)
	stk := clip.Rect(dr).Push(gtx.Ops)
	paint.ColorOp{Color: th.Palette.Divider}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stk.Pop()
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, dims.Size.Y)}
}

func headerName(st *State) string {
	for _, c := range st.Chats {
		if c.JID == st.SelectedChat {
			if c.Name != "" {
				return c.Name
			}
			break
		}
	}
	return st.SelectedChat
}

// layoutMessages renders the message pane with newest-at-bottom
// ordering. state.Messages is stored newest-first (driven by the
// keyset cursor), so we map layout index i to Messages[count-1-i] —
// i=0 becomes the OLDEST loaded message (top of viewport), i=count-1
// the NEWEST (bottom of viewport, anchored by ScrollToEnd).
func (v *View) layoutMessages(gtx layout.Context, th *Theme, st *State) layout.Dimensions {
	mat := th.Material()
	if st.SelectedChat == "" {
		empty := material.Label(mat, th.Type.Body, "Select a chat")
		empty.Color = th.Palette.TextSecondary
		return layout.Center.Layout(gtx, empty.Layout)
	}

	groups := GroupMessages(st.Messages, DefaultGroupWindow)
	count := len(st.Messages)
	ownJID := st.OwnJID
	isGroup := IsGroup(st.SelectedChat)
	v.visiblePaths = v.visiblePaths[:0]
	defer func() {
		// After laying out all visible bubbles, reconcile the cache
		// to match what we just drew. Newly-appearing thumbnails get
		// decoded; departed ones are released so the LRU can evict.
		if v.imgTracker != nil {
			v.imgTracker.SetVisible(v.visiblePaths)
		}
	}()
	return material.List(mat, &v.msgList).Layout(gtx, count, func(gtx layout.Context, i int) layout.Dimensions {
		// Reverse the index so newest sits at the bottom of the viewport.
		idx := count - 1 - i
		m := st.Messages[idx]
		group := groups[idx]
		fromMe := isFromMe(m, ownJID)
		// Sender label: only in group chats, only for received
		// messages (we know who sent ours), only on the first bubble
		// of a sender run (handled inside layoutBubble).
		senderLabel := ""
		if isGroup && !fromMe && m.SenderJID != "" {
			if name := st.NameFor(m.SenderJID); name != "" {
				senderLabel = name
			} else {
				senderLabel = m.SenderJID
			}
		}
		rs := st.Reactions[m.WAID]
		var thumb image.Image
		if m.MediaPath != "" {
			v.visiblePaths = append(v.visiblePaths, m.MediaPath)
			thumb = v.thumbnailFor(m.MediaPath)
		}
		return layoutBubble(gtx, th, m, group, fromMe, senderLabel, rs, thumb)
	})
}

// isFromMe decides whether a message bubble should align right (sent)
// or left (received). When ownJID is set, compare against the sender;
// fall back to "empty sender = from me" for compatibility with the
// seed data and the wa.Handler convention (cmd/seed inserts FromMe
// messages with SenderJID="").
func isFromMe(m store.Message, ownJID string) bool {
	if ownJID != "" {
		return m.SenderJID == ownJID
	}
	return m.SenderJID == ""
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
