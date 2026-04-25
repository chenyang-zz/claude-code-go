package web_fetch

import (
	"container/list"
	"sync"
	"time"
)

// CacheEntry stores one fetched URL result together with metadata needed by the cache layer.
type CacheEntry struct {
	Bytes         int
	Code          int
	CodeText      string
	Content       string
	ContentType   string
	PersistedPath string
	PersistedSize int
}

// Cache implements an LRU cache with TTL and total-size limits for WebFetch results.
type Cache struct {
	mu         sync.Mutex
	entries    map[string]*list.Element
	lru        *list.List
	maxSize    int64
	currentSize int64
	ttl        time.Duration
}

type cacheItem struct {
	key       string
	entry     CacheEntry
	size      int64
	createdAt time.Time
}

// NewCache builds a WebFetch cache with the requested size limit and TTL.
func NewCache(maxSize int64, ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[string]*list.Element),
		lru:     list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get returns a cached entry when it exists and has not expired.
func (c *Cache) Get(url string) (CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.entries == nil {
		return CacheEntry{}, false
	}

	elem, ok := c.entries[url]
	if !ok {
		return CacheEntry{}, false
	}

	item := elem.Value.(*cacheItem)
	if time.Since(item.createdAt) > c.ttl {
		c.removeElement(elem)
		return CacheEntry{}, false
	}

	c.lru.MoveToFront(elem)
	return item.entry, true
}

// Set stores one fetched result under the original URL key.
func (c *Cache) Set(url string, entry CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.entries == nil {
		c.entries = make(map[string]*list.Element)
	}

	// If the key already exists, remove the old entry first so size accounting is correct.
	if elem, ok := c.entries[url]; ok {
		c.removeElement(elem)
	}

	size := int64(len(entry.Content))
	if size < 1 {
		size = 1
	}

	item := &cacheItem{
		key:       url,
		entry:     entry,
		size:      size,
		createdAt: time.Now(),
	}

	elem := c.lru.PushFront(item)
	c.entries[url] = elem
	c.currentSize += size

	c.evict()
}

// Clear removes every entry from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*list.Element)
	c.lru.Init()
	c.currentSize = 0
}

// removeElement unlinks one list element and updates the size accounting.
func (c *Cache) removeElement(elem *list.Element) {
	item := elem.Value.(*cacheItem)
	delete(c.entries, item.key)
	c.lru.Remove(elem)
	c.currentSize -= item.size
	if c.currentSize < 0 {
		c.currentSize = 0
	}
}

// evict drops expired and oversized entries, oldest first.
func (c *Cache) evict() {
	now := time.Now()

	// First pass: remove expired entries.
	for elem := c.lru.Back(); elem != nil; {
		prev := elem.Prev()
		item := elem.Value.(*cacheItem)
		if now.Sub(item.createdAt) > c.ttl {
			c.removeElement(elem)
		}
		elem = prev
	}

	// Second pass: remove oldest entries until we are under the size cap.
	for c.currentSize > c.maxSize && c.lru.Len() > 0 {
		elem := c.lru.Back()
		if elem == nil {
			break
		}
		c.removeElement(elem)
	}
}
