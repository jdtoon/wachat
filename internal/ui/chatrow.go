package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// layoutChatRow renders one row of the chat list: circular avatar with
// initial, name + subtitle stack, trailing time + unread badge.
// Heights are predictable (no measure-everything dependence) so the
// virtualized list can lay out only the visible window cheaply
// (docs/design.md §5 "Predictable row heights").
func layoutChatRow(gtx layout.Context, th *Theme, c ChatSummary) layout.Dimensions {
	mat := th.Material()
	avatarSize := unit.Dp(36)
	if th.Density == DensityCompact {
		avatarSize = unit.Dp(28)
	}

	return layout.Inset{
		Top: th.RowPad(), Bottom: th.RowPad(),
		Left: th.Spacing.M, Right: th.Spacing.M,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
		}.Layout(gtx,
			// Avatar.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutAvatar(gtx, th, c, avatarSize)
			}),
			layout.Rigid(layout.Spacer{Width: th.Spacing.M}.Layout),

			// Name + subtitle stack, flexed to take remaining space.
			// Unread rows bold the name and use a stronger preview
			// color so the eye finds them quickly. Pin / mute glyphs
			// sit next to the name.
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				name := material.Label(mat, th.Type.Title, decorateName(c))
				name.Color = th.Palette.TextPrimary
				name.MaxLines = 1
				sub := material.Label(mat, th.Type.Meta, chatSubtitle(c))
				if c.Unread > 0 {
					name.Font.Weight = font.SemiBold
					sub.Color = th.Palette.TextPrimary
				} else {
					sub.Color = th.Palette.TextSecondary
				}
				sub.MaxLines = 1
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(name.Layout),
					layout.Rigid(sub.Layout),
				)
			}),

			// Trailing: time + unread badge.
			layout.Rigid(layout.Spacer{Width: th.Spacing.S}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutChatTrailing(gtx, th, c)
			}),
		)
	})
}

// layoutAvatar paints a filled circle with a single-letter glyph.
// Background color is deterministic on c.JID so the same chat keeps
// the same avatar tint across launches.
func layoutAvatar(gtx layout.Context, th *Theme, c ChatSummary, size unit.Dp) layout.Dimensions {
	mat := th.Material()
	sz := gtx.Dp(size)
	dims := layout.Dimensions{Size: image.Pt(sz, sz)}

	// Filled circle.
	defer clip.Ellipse(image.Rect(0, 0, sz, sz)).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: avatarColor(c.JID, th)}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	// Initial centered on the circle.
	letter := initial(displayName(c))
	if letter != "" {
		macro := op.Record(gtx.Ops)
		lbl := material.Label(mat, th.Type.Title, letter)
		lbl.Color = th.Palette.AccentText
		lbl.Alignment = text.Middle
		gtx2 := gtx
		gtx2.Constraints = layout.Exact(dims.Size)
		layout.Center.Layout(gtx2, lbl.Layout)
		call := macro.Stop()
		call.Add(gtx.Ops)
	}
	return dims
}

// layoutChatTrailing renders the time on top + unread badge below it.
func layoutChatTrailing(gtx layout.Context, th *Theme, c ChatSummary) layout.Dimensions {
	mat := th.Material()
	t := ""
	if c.LastTS > 0 {
		t = chatTime(c)
	}
	stack := []layout.FlexChild{}
	if t != "" {
		lbl := material.Label(mat, th.Type.Meta, t)
		lbl.Color = th.Palette.TextSecondary
		lbl.MaxLines = 1
		stack = append(stack, layout.Rigid(lbl.Layout))
	}
	if c.Unread > 0 {
		stack = append(stack, layout.Rigid(layout.Spacer{Height: th.Spacing.XXS}.Layout))
		stack = append(stack, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutUnreadBadge(gtx, th, c.Unread)
		}))
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx, stack...)
}

// layoutUnreadBadge is the small accent-colored pill with a count.
func layoutUnreadBadge(gtx layout.Context, th *Theme, count int) layout.Dimensions {
	mat := th.Material()
	lbl := material.Label(mat, th.Type.Meta, formatCount(count))
	lbl.Color = th.Palette.AccentText
	return layout.Inset{
		Top: th.Spacing.XXS, Bottom: th.Spacing.XXS,
		Left: th.Spacing.S, Right: th.Spacing.S,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return roundedFill(gtx, th.Palette.Unread, th.Radius.Button, lbl.Layout)
	})
}

// decorateName prepends pin / mute glyphs to the chat name so the
// state is visible in a glance. The icons stay tiny so the name
// remains the visual anchor.
func decorateName(c ChatSummary) string {
	prefix := ""
	if c.Pinned {
		prefix += "📌 "
	}
	if c.MuteUntil != 0 {
		prefix += "🔇 "
	}
	return prefix + displayName(c)
}

// initial returns the first uppercase letter of name, or empty if name
// has no letters.
func initial(name string) string {
	for _, r := range name {
		if r >= 'a' && r <= 'z' {
			return string(r - 32)
		}
		if r >= 'A' && r <= 'Z' {
			return string(r)
		}
	}
	return ""
}

// chatTime is the trailing-column time string. Today: HH:MM. Older:
// short date. Cheap formatting, no dep.
func chatTime(c ChatSummary) string {
	return humanTime(time.UnixMilli(c.LastTS))
}

func formatCount(n int) string {
	if n > 99 {
		return "99+"
	}
	// itoa without strconv.
	if n == 0 {
		return "0"
	}
	digits := [3]byte{}
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

// avatarColor picks a deterministic hue per chat JID. Returns the
// theme's accent if the JID is empty (defensive — never blank avatar).
func avatarColor(jid string, th *Theme) color.NRGBA {
	if jid == "" {
		return th.Palette.Accent
	}
	// Small FNV-1a-ish hash for stability across launches.
	const offset = 14695981039346656037
	const prime = 1099511628211
	h := uint64(offset)
	for i := 0; i < len(jid); i++ {
		h ^= uint64(jid[i])
		h *= prime
	}
	// Project to a hue around the accent; keep saturation modest so all
	// avatars feel like part of the same palette.
	hue := float64(h%360) / 360.0
	return hsvToRGB(hue, 0.45, 0.85)
}

// hsvToRGB is a tiny conversion (no external dep). v in [0,1].
func hsvToRGB(h, s, v float64) color.NRGBA {
	i := int(h * 6)
	f := h*6 - float64(i)
	p := v * (1 - s)
	q := v * (1 - f*s)
	t := v * (1 - (1-f)*s)
	var r, g, b float64
	switch i % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	case 5:
		r, g, b = v, p, q
	}
	to8 := func(x float64) uint8 {
		if x < 0 {
			x = 0
		}
		if x > 1 {
			x = 1
		}
		return uint8(x * 255)
	}
	return color.NRGBA{R: to8(r), G: to8(g), B: to8(b), A: 0xFF}
}
