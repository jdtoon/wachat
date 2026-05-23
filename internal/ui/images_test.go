package ui

import (
	"testing"
)

func TestMediaTypeGlyph_TruthTable(t *testing.T) {
	cases := []struct {
		kind, glyph, label string
	}{
		{"image", "📷", "Photo"},
		{"video", "🎬", "Video"},
		{"audio", "🎙", "Voice note"},
		{"document", "📄", "Document"},
		{"sticker", "🎴", "Sticker"},
		{"unknown", "📎", "Attachment"},
		{"", "📎", "Attachment"},
	}
	for _, tc := range cases {
		g, l := mediaTypeGlyph(tc.kind)
		if g != tc.glyph || l != tc.label {
			t.Errorf("mediaTypeGlyph(%q) = (%q, %q), want (%q, %q)",
				tc.kind, g, l, tc.glyph, tc.label)
		}
	}
}

func TestNewThumbnailCache_NonNil(t *testing.T) {
	c := NewThumbnailCache()
	if c == nil {
		t.Fatal("NewThumbnailCache returned nil")
	}
	if entries, _ := c.Stats(); entries != 0 {
		t.Errorf("fresh cache should be empty; got %d entries", entries)
	}
}
