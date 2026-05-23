// Package media owns wachat's image/thumbnail cache.
//
// Memory model (CLAUDE.md §6 / §7):
//   - Media bytes are on disk; the DB stores paths.
//   - Thumbnails are decoded only when a row becomes visible (Decode).
//   - When a row scrolls out of view, Release() lets the entry be evicted.
//   - LRU eviction keeps total decoded bytes under the configured budget.
//
// The cache is independent of any actual image format — it takes a
// DecodeFn hook, so unit tests can substitute a trivial decoder and the
// rest of the system isn't bound to a particular image library yet.
package media

import (
	"fmt"
	"image"
	"sync"
)

// DecodeFn decodes a media file at path into a renderable image plus the
// approximate decoded byte cost (for LRU bookkeeping). Returning an
// error indicates the file is unreadable or unsupported; the cache will
// not retry until the next Decode call.
type DecodeFn func(path string) (image.Image, int, error)

// Cache holds decoded thumbnails keyed by media path. Decode reference-
// counts; Release decrements. When total bytes exceed maxBytes, entries
// at refcount 0 are evicted LRU-style.
//
// Safe for concurrent use.
type Cache struct {
	maxBytes int
	decode   DecodeFn

	mu      sync.Mutex
	entries map[string]*entry
	lru     []string // released entries, oldest at index 0
	total   int
}

type entry struct {
	img      image.Image
	bytes    int
	refcount int
}

// New constructs a cache with a byte budget and a decode hook. A budget
// of zero disables eviction (useful for tests that want every Decode
// observable).
func New(maxBytes int, decode DecodeFn) *Cache {
	if decode == nil {
		panic("media.New: DecodeFn must not be nil")
	}
	return &Cache{
		maxBytes: maxBytes,
		decode:   decode,
		entries:  make(map[string]*entry),
	}
}

// Decode returns the cached image for path, calling the DecodeFn the
// first time (and again after eviction). Refcount is incremented; the
// caller must pair every Decode with exactly one Release when the row
// scrolls out of view.
func (c *Cache) Decode(path string) (image.Image, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[path]; ok {
		e.refcount++
		c.removeFromLRU(path)
		return e.img, nil
	}
	img, bytes, err := c.decode(path)
	if err != nil {
		return nil, fmt.Errorf("media.Cache.Decode %q: %w", path, err)
	}
	c.entries[path] = &entry{img: img, bytes: bytes, refcount: 1}
	c.total += bytes
	c.evictIfOver()
	return img, nil
}

// Release decrements the refcount on path. When the count reaches zero
// the entry becomes eligible for LRU eviction.
//
// Release on an unknown path is a no-op — defensive, because the cache
// may have evicted an entry between Decode and Release on a slow path.
func (c *Cache) Release(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[path]
	if !ok {
		return
	}
	if e.refcount > 0 {
		e.refcount--
	}
	if e.refcount == 0 {
		c.removeFromLRU(path)
		c.lru = append(c.lru, path)
		c.evictIfOver()
	}
}

func (c *Cache) removeFromLRU(path string) {
	for i, p := range c.lru {
		if p == path {
			c.lru = append(c.lru[:i], c.lru[i+1:]...)
			return
		}
	}
}

func (c *Cache) evictIfOver() {
	if c.maxBytes <= 0 {
		return
	}
	for c.total > c.maxBytes && len(c.lru) > 0 {
		victim := c.lru[0]
		c.lru = c.lru[1:]
		if e, ok := c.entries[victim]; ok {
			c.total -= e.bytes
			delete(c.entries, victim)
		}
	}
}

// Stats reports the live entry count and total decoded bytes. For tests
// and future observability hooks.
func (c *Cache) Stats() (entries, bytes int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries), c.total
}
