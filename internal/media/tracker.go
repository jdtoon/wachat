package media

// Tracker turns "currently visible paths" into matched Decode / Release
// calls on a Cache. Each SetVisible computes the set difference against
// the previously visible set: newly-appearing paths are Decoded;
// departing paths are Released. Same-set calls are no-ops.
//
// The Tracker is single-goroutine — call only from the UI loop.
type Tracker struct {
	cache   *Cache
	visible map[string]struct{}
}

// NewTracker wires a Tracker to a Cache.
func NewTracker(c *Cache) *Tracker {
	if c == nil {
		panic("media.NewTracker: Cache must not be nil")
	}
	return &Tracker{cache: c, visible: make(map[string]struct{})}
}

// SetVisible reconciles the cache to a new visible set. The slice may
// contain duplicates and empty strings — duplicates are deduplicated;
// empty strings are skipped (rows with no media).
//
// Returns (decoded, released) — the number of cache calls made — so
// tests and metrics can assert the 1:1 invariant.
func (t *Tracker) SetVisible(paths []string) (decoded, released int) {
	next := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		next[p] = struct{}{}
	}

	for p := range t.visible {
		if _, ok := next[p]; !ok {
			t.cache.Release(p)
			released++
		}
	}
	for p := range next {
		if _, ok := t.visible[p]; !ok {
			_, _ = t.cache.Decode(p)
			decoded++
		}
	}
	t.visible = next
	return decoded, released
}

// Clear releases every path the Tracker currently considers visible and
// resets its internal set. Call when switching chats or shutting down.
func (t *Tracker) Clear() (released int) {
	for p := range t.visible {
		t.cache.Release(p)
		released++
	}
	t.visible = make(map[string]struct{})
	return released
}
