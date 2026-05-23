package main

import "testing"

func TestSessionDBPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"wachat.db", "wachat-session.db"},
		{"/tmp/x.db", "/tmp/x-session.db"},
		{"weird-no-suffix", "weird-no-suffix-session"},
	}
	for _, tc := range cases {
		got := sessionDBPath(tc.in)
		if got != tc.want {
			t.Errorf("sessionDBPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
