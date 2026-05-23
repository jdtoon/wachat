package wa

import "testing"

func TestFormatDuration_TruthTable(t *testing.T) {
	cases := []struct {
		secs uint32
		want string
	}{
		{0, ""},
		{1, "0:01"},
		{42, "0:42"},
		{60, "1:00"},
		{61, "1:01"},
		{125, "2:05"},
		{3599, "59:59"},
	}
	for _, tc := range cases {
		if got := FormatDuration(tc.secs); got != tc.want {
			t.Errorf("FormatDuration(%d) = %q, want %q", tc.secs, got, tc.want)
		}
	}
}

func TestFormatBytes_TruthTable(t *testing.T) {
	cases := []struct {
		bytes uint64
		want  string
	}{
		{0, ""},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{2048, "2 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024*1024*2 + 1024*1024/2, "2.5 MB"},
		{uint64(1024) * 1024 * 1024 * 3, "3.0 GB"},
	}
	for _, tc := range cases {
		if got := FormatBytes(tc.bytes); got != tc.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}
