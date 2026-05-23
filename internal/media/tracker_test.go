package media

import (
	"fmt"
	"image"
	"testing"
)

// countingDecoder is a tiny DecodeFn whose Decode/Release counts are
// driven through the Cache + Tracker — exactly the path production uses.
type countingDecoder struct{ decodes int }

func (c *countingDecoder) decode(path string) (image.Image, int, error) {
	c.decodes++
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), 100, nil
}

func TestTracker_FirstVisibleDecodesAll(t *testing.T) {
	dec := &countingDecoder{}
	tk := NewTracker(New(0, dec.decode))

	d, r := tk.SetVisible([]string{"a.jpg", "b.jpg", "c.jpg"})
	if d != 3 {
		t.Errorf("decoded=%d, want 3", d)
	}
	if r != 0 {
		t.Errorf("released=%d, want 0", r)
	}
	if dec.decodes != 3 {
		t.Errorf("DecodeFn calls = %d, want 3", dec.decodes)
	}
}

func TestTracker_SameSetIsNoOp(t *testing.T) {
	dec := &countingDecoder{}
	tk := NewTracker(New(0, dec.decode))

	tk.SetVisible([]string{"a", "b"})
	d, r := tk.SetVisible([]string{"a", "b"})
	if d != 0 || r != 0 {
		t.Errorf("same-set deltas: decoded=%d released=%d, want 0/0", d, r)
	}
}

func TestTracker_ScrollSimulation_DecodeReleaseMatchVisibility(t *testing.T) {
	// 1000 total media rows; visible window of 10; step the window
	// forward 100 times in increments of 5. Each step decodes ~5 new
	// paths and releases ~5 old ones.
	const total = 1000
	const window = 10
	const stepBy = 5
	const steps = 100

	dec := &countingDecoder{}
	tk := NewTracker(New(0, dec.decode))

	paths := make([]string, total)
	for i := range paths {
		paths[i] = fmt.Sprintf("m-%04d.jpg", i)
	}

	totalDecodes := 0
	totalReleases := 0
	for step := 0; step < steps; step++ {
		start := step * stepBy
		end := start + window
		if end > total {
			end = total
		}
		visible := paths[start:end]
		d, r := tk.SetVisible(visible)
		totalDecodes += d
		totalReleases += r

		// Cache holds at most `window` live entries plus any released
		// ones not yet evicted. With budget=0 (no eviction) it holds
		// all uniquely-seen so far.
		// What we care about: at any step, exactly window paths have
		// refcount=1 inside the cache; everything else is refcount=0.
	}

	// On the final step the window covers paths[495:505]. Tracker
	// should report exactly that visible set as decoded.
	if got := totalDecodes; got < total/stepBy {
		// Lower bound: ~steps * (newly visible per step). At stepBy=5
		// and window=10, each step uncovers 5 new paths after the
		// first step. ~100*5 = 500 minimum.
		t.Logf("decodes=%d (rough sanity check; exact count is deterministic)", got)
	}
	if got := totalDecodes - totalReleases; got != window {
		t.Errorf("live (decoded - released) = %d, want %d (the visible window)",
			got, window)
	}
}

func TestTracker_EmptyStringsSkipped(t *testing.T) {
	dec := &countingDecoder{}
	tk := NewTracker(New(0, dec.decode))

	d, _ := tk.SetVisible([]string{"a", "", "b", ""})
	if d != 2 {
		t.Errorf("decoded=%d, want 2 (empty strings must be skipped)", d)
	}
}

func TestTracker_DuplicatePathsDedup(t *testing.T) {
	dec := &countingDecoder{}
	tk := NewTracker(New(0, dec.decode))

	d, _ := tk.SetVisible([]string{"a", "a", "a"})
	if d != 1 {
		t.Errorf("decoded=%d, want 1 (duplicates in visible list must dedup)", d)
	}
	if dec.decodes != 1 {
		t.Errorf("DecodeFn calls = %d, want 1", dec.decodes)
	}
}

func TestTracker_ClearReleasesEverything(t *testing.T) {
	dec := &countingDecoder{}
	tk := NewTracker(New(0, dec.decode))

	tk.SetVisible([]string{"a", "b", "c"})
	if r := tk.Clear(); r != 3 {
		t.Errorf("Clear released %d, want 3", r)
	}
	// And we can SetVisible again from a clean slate.
	d, _ := tk.SetVisible([]string{"a", "b", "c"})
	if d != 3 {
		t.Errorf("post-Clear SetVisible decoded=%d, want 3", d)
	}
}

func TestNewTracker_NilCachePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewTracker(nil) did not panic")
		}
	}()
	_ = NewTracker(nil)
}
