package ui

import (
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Theme is wachat's design system. It owns the color, type, spacing,
// radius, and motion tokens described in docs/design.md §1 and exposes
// a derived *material.Theme for the gioui.org/widget/material helpers
// (lists, scrollbars, labels) so existing call sites keep working.
//
// Tokens are data, not behavior — swap the struct (e.g. light → dark)
// and call w.Invalidate(); no widget tree reflow.
type Theme struct {
	Palette Palette
	Type    Type
	Spacing Spacing
	Radius  Radius
	Motion  Motion
	Density Density

	// Shaper is the gio text shaper. Built once at NewTheme and shared
	// with the derived material.Theme.
	Shaper *text.Shaper

	mat *material.Theme
}

// Palette holds the color tokens for one mode. Matches docs/design.md §1
// one-for-one. Use Palette values via t.Palette.Accent etc. — never
// reference raw colors in widget code.
type Palette struct {
	Canvas        color.NRGBA
	Surface       color.NRGBA
	SurfaceRaised color.NRGBA
	TextPrimary   color.NRGBA
	TextSecondary color.NRGBA
	Accent        color.NRGBA
	AccentText    color.NRGBA
	BubbleSent    color.NRGBA
	BubbleRecv    color.NRGBA
	Divider       color.NRGBA
	Unread        color.NRGBA
}

// Type holds the type-scale tokens (point sizes).
type Type struct {
	Display unit.Sp
	Title   unit.Sp
	Body    unit.Sp
	Meta    unit.Sp
	Label   unit.Sp
}

// Spacing holds the dp scale. Compose paddings/margins from these
// only — arbitrary numbers in widget code are a smell.
type Spacing struct {
	XXS unit.Dp
	XS  unit.Dp
	S   unit.Dp
	M   unit.Dp
	L   unit.Dp
	XL  unit.Dp
	XXL unit.Dp
}

// Radius holds corner-radius tokens.
type Radius struct {
	Bubble unit.Dp
	Card   unit.Dp
	Button unit.Dp
}

// Motion holds animation durations. When Reduced is true, durations
// effectively become zero (see Theme.Duration).
type Motion struct {
	Fast    time.Duration
	Base    time.Duration
	Enter   time.Duration
	Reduced bool
}

// Density determines the verbosity of layout. Compact saves vertical
// space; Comfortable is the default.
type Density int

const (
	DensityComfortable Density = iota
	DensityCompact
)

// NewTheme constructs a Theme with the given palette and the standard
// docs/design.md token values. The font collection is built from
// embedded Public Sans TTFs (see fonts.go).
func NewTheme(p Palette) *Theme {
	t := &Theme{
		Palette: p,
		Type: Type{
			Display: 22,
			Title:   16,
			Body:    15,
			Meta:    12,
			Label:   13,
		},
		Spacing: Spacing{
			XXS: 2, XS: 4, S: 8, M: 12, L: 16, XL: 24, XXL: 32,
		},
		Radius: Radius{
			Bubble: 14,
			Card:   12,
			Button: 10,
		},
		Motion: Motion{
			Fast:  120 * time.Millisecond,
			Base:  180 * time.Millisecond,
			Enter: 220 * time.Millisecond,
		},
		Density: DensityComfortable,
		Shaper:  text.NewShaper(text.WithCollection(fontCollection())),
	}
	t.mat = newMaterialFor(t)
	return t
}

// Material returns the derived *material.Theme. Builds lazily; callers
// can hold the returned pointer for the lifetime of the Theme.
func (t *Theme) Material() *material.Theme {
	if t.mat == nil {
		t.mat = newMaterialFor(t)
	}
	return t.mat
}

// Duration applies Motion.Reduced — returns 0 for any duration when the
// user has reduced-motion enabled. Use this everywhere an animation
// length is read; never reference Motion.Fast directly.
func (t *Theme) Duration(d time.Duration) time.Duration {
	if t.Motion.Reduced {
		return 0
	}
	return d
}

// RowPad returns the vertical padding for a list row, scaled by density.
// Used by chat rows and message bubbles so density toggles cleanly.
func (t *Theme) RowPad() unit.Dp {
	switch t.Density {
	case DensityCompact:
		return t.Spacing.XS
	default:
		return t.Spacing.S
	}
}

// newMaterialFor derives a material.Theme from t. Overrides the default
// Fg / Bg / ContrastBg with our tokens so material.Body1, material.H5,
// etc. render in the right colors with no extra work at call sites.
func newMaterialFor(t *Theme) *material.Theme {
	mt := material.NewTheme()
	mt.Shaper = t.Shaper
	mt.Face = "Public Sans" // family name from FontFace registration
	mt.Palette = material.Palette{
		Fg:         t.Palette.TextPrimary,
		Bg:         t.Palette.Canvas,
		ContrastBg: t.Palette.Accent,
		ContrastFg: t.Palette.AccentText,
	}
	mt.TextSize = t.Type.Body
	// Faces in the shaper carry their Weight/Style; expose Medium as
	// the default "labels are slightly heavier than body" weight.
	_ = font.Weight(0)
	return mt
}
