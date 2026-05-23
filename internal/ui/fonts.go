package ui

import (
	_ "embed"
	"log"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
)

// Embedded Public Sans TTFs (OFL-licensed, see internal/ui/fonts/OFL.txt).
// Three weights cover the design.md type scale: Regular for body, Medium
// for labels, SemiBold for titles.
//
//go:embed fonts/PublicSans-Regular.ttf
var fontPublicSansRegular []byte

//go:embed fonts/PublicSans-Medium.ttf
var fontPublicSansMedium []byte

//go:embed fonts/PublicSans-SemiBold.ttf
var fontPublicSansSemiBold []byte

// FontFamily is the typeface name registered with gio's text shaper.
// material.Theme.Face is set to this so material.Body1 / material.H5
// pick up Public Sans automatically.
const FontFamily = "Public Sans"

// fontCollection parses the embedded Public Sans TTFs into a Gio face
// collection, then appends the bundled Go font collection as a fallback
// (covers emoji + scripts Public Sans doesn't include).
//
// On parse failure we fall back to the bundled Go fonts and log — the
// app should still launch with the wrong typeface rather than panic.
func fontCollection() []font.FontFace {
	out := []font.FontFace{}

	add := func(label string, data []byte, weight font.Weight) {
		face, err := opentype.Parse(data)
		if err != nil {
			log.Printf("wachat: failed to parse embedded %s: %v", label, err)
			return
		}
		out = append(out, font.FontFace{
			Font: font.Font{Typeface: FontFamily, Weight: weight},
			Face: face,
		})
	}

	add("Public Sans Regular", fontPublicSansRegular, font.Normal)
	add("Public Sans Medium", fontPublicSansMedium, font.Medium)
	add("Public Sans SemiBold", fontPublicSansSemiBold, font.SemiBold)

	// Fallback: gofont covers emoji + scripts our embedded TTFs lack.
	out = append(out, gofont.Collection()...)
	return out
}
