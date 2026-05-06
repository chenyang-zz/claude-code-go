package shellprefix

import (
	"container/list"
	"context"
	"sync"
)

const defaultCacheCapacity = 100

// lruCache is a simple LRU cache for shell prefix results.
// It wraps Generate() with a cache that short-circuits Haiku queries
// when the same command has been seen recently.
type lruCache struct {
	mu       sync.RWMutex
	capacity int
	entries  map[string]*list.Element
	order    *list.List
}

// cacheEntry stores one cached prefix result.
type cacheEntry struct {
	key    string
	prefix string
}

var globalLRU = newLRUCache(defaultCacheCapacity)

func newLRUCache(capacity int) *lruCache {
	if capacity <= 0 {
		capacity = defaultCacheCapacity
	}
	return &lruCache{
		capacity: capacity,
		entries:  make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

// get returns the cached prefix for the given command, or "" on miss.
func (c *lruCache) get(command string) string {
	c.mu.RLock()
	elem, ok := c.entries[command]
	if !ok {
		c.mu.RUnlock()
		return ""
	}
	// Move to front (most recently used).
	c.mu.RUnlock()
	c.mu.Lock()
	c.order.MoveToFront(elem)
	c.mu.Unlock()
	return elem.Value.(*cacheEntry).prefix
}

// set stores a prefix result for the given command.
func (c *lruCache) set(command, prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing entry.
	if elem, ok := c.entries[command]; ok {
		c.order.MoveToFront(elem)
		elem.Value.(*cacheEntry).prefix = prefix
		return
	}

	// Evict least recently used when at capacity.
	if c.order.Len() >= c.capacity {
		tail := c.order.Back()
		if tail != nil {
			delete(c.entries, tail.Value.(*cacheEntry).key)
			c.order.Remove(tail)
		}
	}

	elem := c.order.PushFront(&cacheEntry{key: command, prefix: prefix})
	c.entries[command] = elem
}

// GenerateWithCache calls Generate() with an LRU cache lookup.
// Returns a cached result if available (preCheck short-circuit);
// otherwise calls Generate() and caches the result.
func GenerateWithCache(ctx *GenerateCacheCtx) (string, error) {
	if ctx == nil || ctx.Command == "" {
		return "", nil
	}

	if cached := globalLRU.get(ctx.Command); cached != "" {
		return cached, nil
	}

	result, err := Generate(ctx.Ctx, ctx.Command, ctx.PolicySpec)
	if err != nil || result == "" {
		return result, err
	}

	globalLRU.set(ctx.Command, result)
	return result, nil
}

// GenerateCacheCtx carries parameters for GenerateWithCache.
type GenerateCacheCtx struct {
	Ctx        context.Context
	Command    string
	PolicySpec string
}
