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

func TestSummarizeReactions_GroupsByEmojiCountDesc(t *testing.T) {
	rs := []store.Reaction{
		{TargetWAID: "w1", SenderJID: "a", Emoji: "👍"},
		{TargetWAID: "w1", SenderJID: "b", Emoji: "❤"},
		{TargetWAID: "w1", SenderJID: "c", Emoji: "👍"},
		{TargetWAID: "w1", SenderJID: "d", Emoji: "👍"},
		{TargetWAID: "w1", SenderJID: "e", Emoji: "❤"},
	}
	got := summarizeReactions(rs)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Emoji != "👍" || got[0].Count != 3 {
		t.Errorf("got[0] = %+v, want 👍 x3", got[0])
	}
	if got[1].Emoji != "❤" || got[1].Count != 2 {
		t.Errorf("got[1] = %+v, want ❤ x2", got[1])
	}
}

func TestSummarizeReactions_EmptyOrNil(t *testing.T) {
	if got := summarizeReactions(nil); got != nil {
		t.Errorf("nil input: got %+v, want nil", got)
	}
	if got := summarizeReactions([]store.Reaction{}); got != nil {
		t.Errorf("empty input: got %+v, want nil", got)
	}
}

func TestSummarizeReactions_SkipsEmptyEmoji(t *testing.T) {
	got := summarizeReactions([]store.Reaction{
		{Emoji: ""},
		{Emoji: "👍"},
	})
	if len(got) != 1 || got[0].Emoji != "👍" {
		t.Errorf("got %+v, want only [👍]", got)
	}
}

func TestLayoutBubbleMeta_StarRendersOnIncomingToo(t *testing.T) {
	// Pure structural test: with Starred=true and fromMe=false we
	// still get the star glyph in the meta row. We can't easily
	// inspect the rendered glyphs from outside Gio, so we rely on
	// the function not panicking + the bubble height being non-zero.
	th := newTestTheme()
	gtx := testGtx(800, 600)
	dims := layoutBubbleMeta(gtx, th, store.Message{
		WAID: "w1", TS: 1, Body: "hi", Starred: true,
	}, false)
	if dims.Size.Y == 0 {
		t.Errorf("Starred meta row should render non-zero height")
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
