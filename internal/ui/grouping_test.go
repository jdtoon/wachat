package ui

import (
	"testing"
	"time"

	"github.com/jdtoon/wachat/internal/store"
)

// m is a tiny constructor for grouping tests. ts is unix seconds (we
// convert to millis internally so the test reads naturally).
func m(sender string, ts int64) store.Message {
	return store.Message{SenderJID: sender, TS: ts * 1000}
}

func TestGroupMessages_SingleSolo(t *testing.T) {
	got := GroupMessages([]store.Message{m("alice", 100)}, time.Minute)
	want := []GroupPosition{GroupSolo}
	if !equalGroups(got, want) {
		t.Errorf("single message: got %v, want %v", got, want)
	}
}

func TestGroupMessages_DifferentSendersAllSolo(t *testing.T) {
	// newest-first: alice at t=200, bob at t=100
	msgs := []store.Message{m("alice", 200), m("bob", 100)}
	got := GroupMessages(msgs, time.Minute)
	want := []GroupPosition{GroupSolo, GroupSolo}
	if !equalGroups(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupMessages_SameSenderWithinWindow(t *testing.T) {
	// Three alice messages within 30s of each other. newest-first storage:
	// idx 0: newest (t=200)
	// idx 1: middle  (t=180)
	// idx 2: oldest  (t=160)
	// In display order (top→bottom): idx 2 → 1 → 0.
	msgs := []store.Message{
		m("alice", 200),
		m("alice", 180),
		m("alice", 160),
	}
	got := GroupMessages(msgs, time.Minute)
	want := []GroupPosition{GroupTail, GroupMiddle, GroupHead}
	if !equalGroups(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupMessages_TwoMessagesSameSender(t *testing.T) {
	msgs := []store.Message{
		m("alice", 100),
		m("alice", 90),
	}
	got := GroupMessages(msgs, time.Minute)
	want := []GroupPosition{GroupTail, GroupHead}
	if !equalGroups(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupMessages_GapExceedsWindowBreaksGroup(t *testing.T) {
	// alice at t=200, alice at t=60 (gap of 140s > 60s window → break).
	msgs := []store.Message{m("alice", 200), m("alice", 60)}
	got := GroupMessages(msgs, time.Minute)
	want := []GroupPosition{GroupSolo, GroupSolo}
	if !equalGroups(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupMessages_DifferentSenderBreaksGroup(t *testing.T) {
	// alice, bob between two alices — middle bob breaks the run.
	msgs := []store.Message{
		m("alice", 200),
		m("bob", 190),
		m("alice", 180),
	}
	got := GroupMessages(msgs, time.Minute)
	want := []GroupPosition{GroupSolo, GroupSolo, GroupSolo}
	if !equalGroups(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupMessages_TwoGroupsBackToBack(t *testing.T) {
	// alice has 3 messages, then bob has 2 — newest-first.
	msgs := []store.Message{
		m("bob", 300),
		m("bob", 290),
		m("alice", 200),
		m("alice", 195),
		m("alice", 190),
	}
	got := GroupMessages(msgs, time.Minute)
	want := []GroupPosition{
		GroupTail, GroupHead, // bob group
		GroupTail, GroupMiddle, GroupHead, // alice group
	}
	if !equalGroups(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupMessages_EmptyInput(t *testing.T) {
	got := GroupMessages(nil, time.Minute)
	if len(got) != 0 {
		t.Errorf("empty input: got %v, want empty slice", got)
	}
}

func TestGroupMessages_DefaultWindowIsFiveMinutes(t *testing.T) {
	if DefaultGroupWindow != 5*time.Minute {
		t.Errorf("DefaultGroupWindow = %v, want 5 minutes (design.md §3)", DefaultGroupWindow)
	}
}

func equalGroups(a, b []GroupPosition) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
