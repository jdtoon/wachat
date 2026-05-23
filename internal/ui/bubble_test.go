package ui

import (
	"testing"

	"github.com/jdtoon/wachat/internal/store"
)

func TestBubbleBodyText_Plain(t *testing.T) {
	text, italic := bubbleBodyText(store.Message{Body: "hi"})
	if text != "hi" || italic {
		t.Errorf("plain: got (%q, %v), want (\"hi\", false)", text, italic)
	}
}

func TestBubbleBodyText_Edited(t *testing.T) {
	text, italic := bubbleBodyText(store.Message{Body: "hello", Edited: true})
	want := "hello  (edited)"
	if text != want || italic {
		t.Errorf("edited: got (%q, %v), want (%q, false)", text, italic, want)
	}
}

func TestBubbleBodyText_EditedEmpty(t *testing.T) {
	text, italic := bubbleBodyText(store.Message{Edited: true})
	if text != "(edited)" || !italic {
		t.Errorf("edited+empty: got (%q, %v), want (\"(edited)\", true)", text, italic)
	}
}

func TestBubbleBodyText_RevokedOverridesEverything(t *testing.T) {
	text, italic := bubbleBodyText(store.Message{Body: "real text", Edited: true, Revoked: true})
	if italic != true {
		t.Errorf("revoked should be italic; got %v", italic)
	}
	if text == "real text" {
		t.Errorf("revoked must not show original body; got %q", text)
	}
}

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
