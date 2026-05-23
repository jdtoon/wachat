package ui

import (
	"image"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/jdtoon/wachat/internal/store"
)

// SearchBar holds the search input and any per-result click state. Lives
// next to the chat list so the user can scope their attention to one
// pane at a time.
type SearchBar struct {
	editor    widget.Editor
	hitClicks []widget.Clickable
	hitList   widget.List
}

// NewSearchBar constructs a SearchBar with a single-line editor.
func NewSearchBar() *SearchBar {
	sb := &SearchBar{}
	sb.editor.SingleLine = true
	sb.editor.Submit = true // Enter fires submit; we listen for the event
	sb.hitList.Axis = layout.Vertical
	return sb
}

// Query returns the current input text, trimmed.
func (s *SearchBar) Query() string { return strings.TrimSpace(s.editor.Text()) }

// Clear empties the input. Use after a chat is selected if you want
// the bar to reset (UX choice — we don't auto-clear so the user can
// click multiple hits).
func (s *SearchBar) Clear() { s.editor.SetText("") }

// LayoutInput renders just the search input strip. Returns the
// dimensions and, if the user submitted (Enter or button), the trimmed
// query via onSubmit.
func (s *SearchBar) LayoutInput(gtx layout.Context, th *Theme, onSubmit func(query string)) layout.Dimensions {
	mat := th.Material()
	// Editor submit event → onSubmit.
	if onSubmit != nil {
		for {
			ev, ok := gtx.Event(key.Filter{Focus: &s.editor, Name: key.NameReturn})
			if !ok {
				break
			}
			if ke, ok := ev.(key.Event); ok && ke.State == key.Press {
				onSubmit(s.Query())
			}
		}
	}
	return paintHeaderSurface(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: th.Spacing.S, Bottom: th.Spacing.S,
			Left: th.Spacing.M, Right: th.Spacing.M,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(mat, &s.editor, "Search messages…")
			ed.Color = th.Palette.TextPrimary
			ed.HintColor = th.Palette.TextSecondary
			ed.TextSize = th.Type.Body
			return ed.Layout(gtx)
		})
	})
}

// LayoutResults renders the search-hit overlay below the input. Each
// row shows chat name, snippet, and time. Clicking a row invokes
// onJump with the hit so the caller can call state.JumpToMessage.
//
// If there are no results (st.Results == nil) the function returns
// zero dimensions so the caller can skip rendering it.
func (s *SearchBar) LayoutResults(gtx layout.Context, th *Theme, st *State, onJump func(hit store.SearchHit)) layout.Dimensions {
	if st.Results == nil {
		return layout.Dimensions{}
	}
	mat := th.Material()
	if len(s.hitClicks) != len(st.Results) {
		s.hitClicks = make([]widget.Clickable, len(st.Results))
	}
	if onJump != nil {
		for i := range s.hitClicks {
			if s.hitClicks[i].Clicked(gtx) {
				onJump(st.Results[i])
			}
		}
	}

	if len(st.Results) == 0 {
		empty := material.Label(mat, th.Type.Meta, "no matches")
		empty.Color = th.Palette.TextSecondary
		return layout.Inset{
			Top: th.Spacing.S, Bottom: th.Spacing.S,
			Left: th.Spacing.M, Right: th.Spacing.M,
		}.Layout(gtx, empty.Layout)
	}

	return paintSurface(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return material.List(mat, &s.hitList).Layout(gtx, len(st.Results), func(gtx layout.Context, i int) layout.Dimensions {
			hit := st.Results[i]
			return s.hitClicks[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layoutSearchHitRow(gtx, th, hit)
			})
		})
	})
}

// layoutSearchHitRow renders one row of the result overlay.
func layoutSearchHitRow(gtx layout.Context, th *Theme, hit store.SearchHit) layout.Dimensions {
	mat := th.Material()
	chat := hit.ChatName
	if chat == "" {
		chat = hit.ChatJID
	}

	return layout.Inset{
		Top: th.RowPad(), Bottom: th.RowPad(),
		Left: th.Spacing.M, Right: th.Spacing.M,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		name := material.Label(mat, th.Type.Label, chat)
		name.Color = th.Palette.TextPrimary
		name.MaxLines = 1
		snippet := layoutSnippet(gtx, th, hit.Snippet)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(name.Layout),
			layout.Rigid(layout.Spacer{Height: th.Spacing.XXS}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return snippet }),
		)
	})
}

// layoutSnippet renders the search-result snippet. The [[ … ]] markers
// (FTS5 snippet output) are turned into a tinted accent run so the
// match is visually distinct from the surrounding text.
//
// Light-weight implementation: we draw the whole snippet as one Label
// for now (the accent highlighting comes once Gio's text shaper grows
// inline color runs in a richer version). The brackets are stripped so
// they don't show literally.
func layoutSnippet(gtx layout.Context, th *Theme, snippet string) layout.Dimensions {
	mat := th.Material()
	cleaned := strings.NewReplacer("[[", "", "]]", "").Replace(snippet)
	lbl := material.Label(mat, th.Type.Meta, cleaned)
	lbl.Color = th.Palette.TextSecondary
	lbl.MaxLines = 2
	lbl.Alignment = text.Start
	return lbl.Layout(gtx)
}

// paintSurface fills with Theme.Palette.Surface behind the inset
// content. Used by the search-results overlay so it visually sits on
// top of the chat list while sharing the sidebar background.
func paintSurface(gtx layout.Context, th *Theme, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := w(gtx)
	call := macro.Stop()
	rect := image.Rect(0, 0, gtx.Constraints.Max.X, dims.Size.Y)
	defer clip.Rect(rect).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: th.Palette.Surface}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, dims.Size.Y)}
}
