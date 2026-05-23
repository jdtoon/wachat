package ui

import "testing"

func TestSetTyping_AddsAndExpires(t *testing.T) {
	st := &State{}
	now := int64(1_000_000)
	st.SetTyping("c1", "alice", true, now)
	if len(st.ActiveTypers("c1", now)) != 1 {
		t.Fatalf("after compose: want 1 active, got %d", len(st.ActiveTypers("c1", now)))
	}
	// Past the TTL.
	later := now + TypingTTL + 1
	if len(st.ActiveTypers("c1", later)) != 0 {
		t.Errorf("after TTL: want 0 active, got %d", len(st.ActiveTypers("c1", later)))
	}
}

func TestSetTyping_PauseRemovesEntry(t *testing.T) {
	st := &State{}
	now := int64(1_000_000)
	st.SetTyping("c1", "alice", true, now)
	st.SetTyping("c1", "alice", false, now+50)
	if got := st.ActiveTypers("c1", now+100); len(got) != 0 {
		t.Errorf("after pause: %+v", got)
	}
}

func TestSetTyping_MultipleSendersCoexist(t *testing.T) {
	st := &State{}
	now := int64(1_000_000)
	st.SetTyping("c1", "alice", true, now)
	st.SetTyping("c1", "bob", true, now+10)
	if got := st.ActiveTypers("c1", now+20); len(got) != 2 {
		t.Errorf("two senders should coexist; got %+v", got)
	}
}

func TestSetTyping_RepeatedComposeExtendsDeadline(t *testing.T) {
	st := &State{}
	now := int64(1_000_000)
	st.SetTyping("c1", "alice", true, now)
	// Re-fire later (user keeps typing).
	st.SetTyping("c1", "alice", true, now+5000)
	if got := st.ActiveTypers("c1", now+TypingTTL+1); len(got) != 1 {
		t.Errorf("repeated compose should refresh TTL; got %d active", len(got))
	}
}

func TestSetTyping_EmptyArgsAreNoOp(t *testing.T) {
	st := &State{}
	st.SetTyping("", "alice", true, 1)
	st.SetTyping("c1", "", true, 1)
	if len(st.Typing) != 0 {
		t.Errorf("empty-arg call mutated map: %+v", st.Typing)
	}
}

func TestJoinNames_TruthTable(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a and b"},
		{[]string{"a", "b", "c"}, "a, b, and c"},
		{[]string{"a", "b", "c", "d"}, "a, b, c, and d"},
	}
	for _, tc := range cases {
		if got := joinNames(tc.in); got != tc.want {
			t.Errorf("joinNames(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
