package ui

// DarkPalette is the wachat dark-mode palette from docs/design.md §1.
// True-OLED: Canvas is pure black so OLED displays save power and
// elements float on a deep neutral.
var DarkPalette = Palette{
	Canvas:        nrgba(0x000000),
	Surface:       nrgba(0x0E120F),
	SurfaceRaised: nrgba(0x171C18),
	TextPrimary:   nrgba(0xECEFEA),
	TextSecondary: nrgba(0x8A9187),
	Accent:        nrgba(0x3FB68A),
	AccentText:    nrgba(0x04130C),
	BubbleSent:    nrgba(0x13402F),
	BubbleRecv:    nrgba(0x171C18),
	Divider:       nrgba(0x23291F),
	Unread:        nrgba(0x3FB68A),
}
