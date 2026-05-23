package ui

import "image/color"

// LightPalette is the wachat light-mode palette from docs/design.md §1.
// "Refined minimalism, content-first" — a near-neutral canvas with one
// confident accent.
var LightPalette = Palette{
	Canvas:        nrgba(0xF4F5F3),
	Surface:       nrgba(0xFFFFFF),
	SurfaceRaised: nrgba(0xFFFFFF), // raised look comes from shadow, not fill
	TextPrimary:   nrgba(0x11140F),
	TextSecondary: nrgba(0x6B7167),
	Accent:        nrgba(0x1F8F6B),
	AccentText:    nrgba(0xFFFFFF),
	BubbleSent:    nrgba(0xD6F3E4),
	BubbleRecv:    nrgba(0xFFFFFF),
	Divider:       nrgba(0xE4E6E1),
	Unread:        nrgba(0x1F8F6B),
}

// nrgba parses a 0xRRGGBB literal into an opaque color.NRGBA. Keeps the
// palette tables compact and easy to compare against docs/design.md.
func nrgba(rgb uint32) color.NRGBA {
	return color.NRGBA{
		R: uint8(rgb >> 16),
		G: uint8(rgb >> 8),
		B: uint8(rgb),
		A: 0xFF,
	}
}
