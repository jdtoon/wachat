package media

import (
	"errors"
	"image"
	"testing"
)

// recordingDecoder is a DecodeFn that returns a 1x1 image and records
// every call.
type recordingDecoder struct {
	calls []string
	err   error
	bytes int // bytes reported per call; default 100
}

func (r *recordingDecoder) decode(path string) (image.Image, int, error) {
	r.calls = append(r.calls, path)
	if r.err != nil {
		return nil, 0, r.err
	}
	b := r.bytes
	if b == 0 {
		b = 100
	}
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), b, nil
}

func TestCache_DecodeCachesAcrossCalls(t *testing.T) {
	dec := &recordingDecoder{}
	c := New(0, dec.decode)

	if _, err := c.Decode("a.jpg"); err != nil {
		t.Fatalf("Decode #1: %v", err)
	}
	if _, err := c.Decode("a.jpg"); err != nil {
		t.Fatalf("Decode #2: %v", err)
	}

	if got := len(dec.calls); got != 1 {
		t.Errorf("DecodeFn calls = %d, want 1 (second Decode must use cache)", got)
	}
}

func TestCache_DecodeErrorIsReturned(t *testing.T) {
	sentinel := errors.New("unreadable")
	dec := &recordingDecoder{err: sentinel}
	c := New(0, dec.decode)

	_, err := c.Decode("bad.jpg")
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of %v", err, sentinel)
	}
	if n, _ := c.Stats(); n != 0 {
		t.Errorf("entries after failed decode = %d, want 0", n)
	}
}

func TestCache_ReleaseDecRefcount(t *testing.T) {
	dec := &recordingDecoder{}
	c := New(0, dec.decode)

	_, _ = c.Decode("a")
	_, _ = c.Decode("a")
	c.Release("a")
	// Still 1 refcount → must not be evicted.
	if n, _ := c.Stats(); n != 1 {
		t.Errorf("entries after partial release = %d, want 1", n)
	}
}

func TestCache_LRUEvictsReleasedEntries(t *testing.T) {
	dec := &recordingDecoder{bytes: 100}
	c := New(250, dec.decode) // budget = 2.5 entries

	for _, p := range []string{"a", "b", "c"} {
		_, _ = c.Decode(p)
		c.Release(p) // refcount=0 immediately
	}

	// Total 300 > budget 250 → oldest released ("a") must be evicted.
	entries, _ := c.Stats()
	if entries != 2 {
		t.Errorf("entries after over-budget releases = %d, want 2", entries)
	}

	// Decoding "a" again triggers a fresh DecodeFn call.
	before := len(dec.calls)
	_, _ = c.Decode("a")
	if len(dec.calls) != before+1 {
		t.Errorf("DecodeFn calls after re-Decode of evicted = %d, want %d",
			len(dec.calls), before+1)
	}
}

func TestCache_LiveEntriesNotEvicted(t *testing.T) {
	dec := &recordingDecoder{bytes: 100}
	c := New(150, dec.decode) // budget = 1.5 entries

	_, _ = c.Decode("live") // refcount=1, NOT released
	_, _ = c.Decode("released")
	c.Release("released")

	// Total 200 > budget 150 → can only evict the released one.
	if _, ok := lookup(c, "live"); !ok {
		t.Error("live entry was evicted; LRU must not evict refcount>0")
	}
	if _, ok := lookup(c, "released"); ok {
		t.Error("released entry should have been evicted under budget pressure")
	}
}

func TestCache_ReleaseUnknownIsNoOp(t *testing.T) {
	c := New(0, (&recordingDecoder{}).decode)
	c.Release("never-decoded") // must not panic
	if n, _ := c.Stats(); n != 0 {
		t.Errorf("entries = %d after no-op release", n)
	}
}

func TestNew_NilDecodeFnPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("New(_, nil) did not panic")
		}
	}()
	_ = New(100, nil)
}

// lookup is a test helper that peeks at whether the cache currently
// holds an entry for path. Touches no public API.
func lookup(c *Cache, path string) (image.Image, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[path]
	if !ok {
		return nil, false
	}
	return e.img, true
}
