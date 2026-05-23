package ui

import (
	"time"

	"github.com/jdtoon/wachat/internal/store"
)

// GroupPosition classifies a message's place in a visually-grouped run
// of consecutive messages from the same sender within a small time
// window. Used by the bubble layout to soften corners and collapse
// repeated avatars/headers (docs/design.md §3).
type GroupPosition uint8

const (
	// GroupSolo: the message stands alone — no neighbour in the same group.
	GroupSolo GroupPosition = iota
	// GroupHead: oldest message of a group (rendered at the top of the run).
	GroupHead
	// GroupMiddle: middle of a multi-message group.
	GroupMiddle
	// GroupTail: newest message of a group (rendered at the bottom of the
	// run, carries the timestamp + receipt).
	GroupTail
)

// DefaultGroupWindow is the maximum gap between two consecutive messages
// from the same sender that still count as one visual group.
const DefaultGroupWindow = 5 * time.Minute

// GroupMessages classifies each entry in msgs by its position in a
// time-bounded sender run. The input is newest-first (state.Messages
// ordering); the returned slice is parallel and uses the same indexing.
//
// Pure function — no Gio, no store calls. Tested via a table in
// grouping_test.go.
func GroupMessages(msgs []store.Message, window time.Duration) []GroupPosition {
	out := make([]GroupPosition, len(msgs))
	winMs := window.Milliseconds()

	// "Above" in display order = older = msgs[i+1] (because msgs is
	// newest-first storage). "Below" = newer = msgs[i-1].
	sameGroup := func(a, b store.Message) bool {
		if a.SenderJID != b.SenderJID {
			return false
		}
		gap := a.TS - b.TS
		if gap < 0 {
			gap = -gap
		}
		return gap <= winMs
	}

	for i := range msgs {
		hasAbove := false
		hasBelow := false
		if i+1 < len(msgs) {
			hasAbove = sameGroup(msgs[i], msgs[i+1])
		}
		if i > 0 {
			hasBelow = sameGroup(msgs[i], msgs[i-1])
		}
		switch {
		case !hasAbove && !hasBelow:
			out[i] = GroupSolo
		case !hasAbove && hasBelow:
			out[i] = GroupHead
		case hasAbove && hasBelow:
			out[i] = GroupMiddle
		case hasAbove && !hasBelow:
			out[i] = GroupTail
		}
	}
	return out
}
