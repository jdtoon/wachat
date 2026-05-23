package ui

import (
	"testing"

	"github.com/jdtoon/wachat/internal/store"
)

func TestReceiptGlyph_TruthTable(t *testing.T) {
	cases := []struct {
		status string
		want   string
		accent bool
	}{
		{store.StatusPending, "⏱", false},
		{store.StatusSent, "✓", false},
		{store.StatusDelivered, "✓✓", false},
		{store.StatusRead, "✓✓", true},
		{store.StatusPlayed, "✓✓", true},
		{"unknown", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			g, a := receiptGlyph(tc.status)
			if g != tc.want {
				t.Errorf("glyph = %q, want %q", g, tc.want)
			}
			if a != tc.accent {
				t.Errorf("accent = %v, want %v", a, tc.accent)
			}
		})
	}
}
