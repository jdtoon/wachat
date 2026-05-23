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

func TestLinkHost_ParsesURL(t *testing.T) {
	if got := linkHost("https://example.com/some/path?q=1"); got != "example.com" {
		t.Errorf("linkHost full URL = %q, want example.com", got)
	}
}

func TestLinkHost_FallsBackToTruncatedString(t *testing.T) {
	url := "not-a-valid-url-but-also-very-long-so-it-should-be-truncated-with-an-ellipsis"
	got := linkHost(url)
	if len(got) > 65 {
		t.Errorf("linkHost should truncate long invalid URLs; got %d chars", len(got))
	}
}

func TestLinkHost_Empty(t *testing.T) {
	if got := linkHost(""); got != "" {
		t.Errorf("linkHost('') = %q, want empty", got)
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
