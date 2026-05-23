package ui

import (
	"image/color"
	"math"
	"testing"
)

func TestNewTheme_DefaultTokensAreSensible(t *testing.T) {
	th := NewTheme(LightPalette)

	if th.Type.Body <= 0 {
		t.Errorf("Type.Body = %v, want > 0", th.Type.Body)
	}
	if th.Type.Display <= th.Type.Title || th.Type.Title <= th.Type.Body {
		t.Errorf("type scale not monotonic: display=%v title=%v body=%v",
			th.Type.Display, th.Type.Title, th.Type.Body)
	}

	if th.Spacing.S <= th.Spacing.XS || th.Spacing.L <= th.Spacing.M {
		t.Errorf("spacing scale not monotonic: %+v", th.Spacing)
	}

	if th.Radius.Bubble == 0 || th.Radius.Card == 0 || th.Radius.Button == 0 {
		t.Errorf("radius tokens have zero values: %+v", th.Radius)
	}

	if th.Motion.Fast == 0 || th.Motion.Base == 0 || th.Motion.Enter == 0 {
		t.Errorf("motion tokens have zero values: %+v", th.Motion)
	}
	if th.Motion.Reduced {
		t.Errorf("Motion.Reduced default should be false")
	}

	if th.Density != DensityComfortable {
		t.Errorf("default density = %v, want DensityComfortable", th.Density)
	}

	if th.Shaper == nil {
		t.Errorf("Shaper is nil")
	}
}

func TestNewTheme_MaterialDerivationReturnsUsableTheme(t *testing.T) {
	th := NewTheme(LightPalette)
	mat := th.Material()
	if mat == nil {
		t.Fatal("Material() returned nil")
	}
	if mat.Shaper != th.Shaper {
		t.Error("material.Theme.Shaper does not match our Shaper")
	}
	if mat.TextSize != th.Type.Body {
		t.Errorf("material.Theme.TextSize = %v, want %v", mat.TextSize, th.Type.Body)
	}
	if mat.Palette.Fg != th.Palette.TextPrimary {
		t.Errorf("material Fg = %v, want %v (TextPrimary)", mat.Palette.Fg, th.Palette.TextPrimary)
	}
	if mat.Palette.ContrastBg != th.Palette.Accent {
		t.Errorf("material ContrastBg = %v, want %v (Accent)", mat.Palette.ContrastBg, th.Palette.Accent)
	}
}

func TestTheme_DurationRespectsReducedMotion(t *testing.T) {
	th := NewTheme(LightPalette)
	if got := th.Duration(th.Motion.Base); got != th.Motion.Base {
		t.Errorf("with Reduced=false, Duration returned %v, want %v", got, th.Motion.Base)
	}

	th.Motion.Reduced = true
	if got := th.Duration(th.Motion.Base); got != 0 {
		t.Errorf("with Reduced=true, Duration returned %v, want 0", got)
	}
}

func TestTheme_RowPadShrinksOnCompactDensity(t *testing.T) {
	th := NewTheme(LightPalette)
	comfortable := th.RowPad()

	th.Density = DensityCompact
	compact := th.RowPad()
	if compact >= comfortable {
		t.Errorf("compact row pad (%v) should be smaller than comfortable (%v)",
			compact, comfortable)
	}
}

// TestPalettes_BodyTextHasWCAGAAContrast asserts the readability of
// the combinations that carry body-sized text. WCAG AA requires ≥
// 4.5:1 for body text and ≥ 3.0:1 for large text (≥ 18pt or ≥ 14pt
// bold — covers button labels). AccentText-on-Accent is used for
// button/badge labels (Label token = 13sp Medium), so AA Large is the
// right standard there; bubble body text uses TextPrimary on the
// BubbleSent / BubbleRecv backgrounds and must meet AA body.
func TestPalettes_BodyTextHasWCAGAAContrast(t *testing.T) {
	cases := []struct {
		name string
		p    Palette
	}{
		{"light", LightPalette},
		{"dark", DarkPalette},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := func(fg, bg color.NRGBA, label string) {
				if r := contrastRatio(fg, bg); r < 4.5 {
					t.Errorf("%s contrast = %.2f, want ≥ 4.5 (AA body)", label, r)
				}
			}
			large := func(fg, bg color.NRGBA, label string) {
				if r := contrastRatio(fg, bg); r < 3.0 {
					t.Errorf("%s contrast = %.2f, want ≥ 3.0 (AA large)", label, r)
				}
			}

			body(tc.p.TextPrimary, tc.p.Canvas, "TextPrimary on Canvas")
			body(tc.p.TextPrimary, tc.p.Surface, "TextPrimary on Surface")
			body(tc.p.TextPrimary, tc.p.BubbleSent, "TextPrimary on BubbleSent")
			body(tc.p.TextPrimary, tc.p.BubbleRecv, "TextPrimary on BubbleRecv")
			large(tc.p.AccentText, tc.p.Accent, "AccentText on Accent (button labels)")
		})
	}
}

// TestPalettes_LightAndDarkAreDistinct guards against accidentally
// pointing both modes at the same struct.
func TestPalettes_LightAndDarkAreDistinct(t *testing.T) {
	if LightPalette.Canvas == DarkPalette.Canvas {
		t.Error("light and dark Canvas are the same color")
	}
	if LightPalette.TextPrimary == DarkPalette.TextPrimary {
		t.Error("light and dark TextPrimary are the same color")
	}
}

// --- WCAG contrast helpers (only used by tests) ---

// relativeLuminance per WCAG 2.x: convert sRGB to linear, then luminance.
func relativeLuminance(c color.NRGBA) float64 {
	lin := func(v uint8) float64 {
		f := float64(v) / 255.0
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

func contrastRatio(a, b color.NRGBA) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
