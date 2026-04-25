package web_fetch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_GetMiss(t *testing.T) {
	c := NewCache(1024, time.Minute)
	_, ok := c.Get("https://example.com")
	assert.False(t, ok)
}

func TestCache_SetAndGet(t *testing.T) {
	c := NewCache(1024, time.Minute)
	entry := CacheEntry{Content: "hello", Bytes: 5, Code: 200}
	c.Set("https://example.com", entry)

	got, ok := c.Get("https://example.com")
	require.True(t, ok)
	assert.Equal(t, "hello", got.Content)
	assert.Equal(t, 200, got.Code)
}

func TestCache_TTLEviction(t *testing.T) {
	c := NewCache(1024, time.Millisecond)
	c.Set("https://example.com", CacheEntry{Content: "x", Bytes: 1})

	time.Sleep(2 * time.Millisecond)
	_, ok := c.Get("https://example.com")
	assert.False(t, ok)
}

func TestCache_SizeEviction(t *testing.T) {
	c := NewCache(10, time.Minute)
	c.Set("https://a.com", CacheEntry{Content: "12345", Bytes: 5})
	c.Set("https://b.com", CacheEntry{Content: "67890", Bytes: 5})
	// Should still fit (10 bytes)
	_, ok := c.Get("https://a.com")
	assert.True(t, ok)
	_, ok = c.Get("https://b.com")
	assert.True(t, ok)

	// Add one more byte to trigger eviction
	c.Set("https://c.com", CacheEntry{Content: "X", Bytes: 1})
	// Oldest (a.com) should be evicted
	_, ok = c.Get("https://a.com")
	assert.False(t, ok)
	_, ok = c.Get("https://b.com")
	assert.True(t, ok)
	_, ok = c.Get("https://c.com")
	assert.True(t, ok)
}

func TestCache_Clear(t *testing.T) {
	c := NewCache(1024, time.Minute)
	c.Set("https://example.com", CacheEntry{Content: "hello"})
	c.Clear()
	_, ok := c.Get("https://example.com")
	assert.False(t, ok)
}

func TestCache_UpdateExistingKey(t *testing.T) {
	c := NewCache(1024, time.Minute)
	c.Set("https://example.com", CacheEntry{Content: "first", Bytes: 5})
	c.Set("https://example.com", CacheEntry{Content: "second", Bytes: 6})

	got, ok := c.Get("https://example.com")
	require.True(t, ok)
	assert.Equal(t, "second", got.Content)
}
