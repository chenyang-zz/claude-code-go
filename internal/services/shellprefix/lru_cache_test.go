package shellprefix

import (
	"context"
	"testing"
)

func TestCacheGetSet(t *testing.T) {
	c := newLRUCache(10)
	c.set("cmd1", "prefix1")
	if got := c.get("cmd1"); got != "prefix1" {
		t.Errorf("get after set: got %q, want %q", got, "prefix1")
	}
}

func TestCacheMiss(t *testing.T) {
	c := newLRUCache(10)
	if got := c.get("nonexistent"); got != "" {
		t.Errorf("get miss: got %q, want empty", got)
	}
}

func TestCacheEviction(t *testing.T) {
	c := newLRUCache(2)
	c.set("a", "pa")
	c.set("b", "pb")
	c.set("c", "pc") // should evict "a"

	if got := c.get("a"); got != "" {
		t.Errorf("evicted entry 'a' still present: got %q", got)
	}
	if got := c.get("b"); got != "pb" {
		t.Errorf("retained entry 'b': got %q, want %q", got, "pb")
	}
	if got := c.get("c"); got != "pc" {
		t.Errorf("new entry 'c': got %q, want %q", got, "pc")
	}
}

func TestCacheLRUOrder(t *testing.T) {
	c := newLRUCache(2)
	c.set("a", "pa")
	c.set("b", "pb")

	// Access "a" — makes it most recently used.
	c.get("a")
	c.set("c", "pc") // should evict "b" (least recently used)

	if got := c.get("b"); got != "" {
		t.Errorf("evicted entry 'b' still present: got %q", got)
	}
	if got := c.get("a"); got != "pa" {
		t.Errorf("retained entry 'a': got %q, want %q", got, "pa")
	}
}

func TestCacheUpdateExisting(t *testing.T) {
	c := newLRUCache(10)
	c.set("cmd", "old")
	c.set("cmd", "new")

	if got := c.get("cmd"); got != "new" {
		t.Errorf("updated entry: got %q, want %q", got, "new")
	}
}

func TestGenerateWithCacheShortCircuit(t *testing.T) {
	// When cache has an entry, preCheck should return it without calling Generate.
	c := newLRUCache(10)
	c.set("test-command", "test-prefix")

	// Temporarily replace globalLRU.
	orig := globalLRU
	globalLRU = c
	defer func() { globalLRU = orig }()

	result, err := GenerateWithCache(&GenerateCacheCtx{
		Ctx:     context.Background(),
		Command: "test-command",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != "test-prefix" {
		t.Errorf("short-circuit: got %q, want %q", result, "test-prefix")
	}
}

func TestGenerateWithCacheEmptyCommand(t *testing.T) {
	result, err := GenerateWithCache(nil)
	if err != nil {
		t.Errorf("nil ctx should return nil, got: %v", err)
	}
	if result != "" {
		t.Errorf("nil ctx should return empty, got: %q", result)
	}

	result, err = GenerateWithCache(&GenerateCacheCtx{
		Ctx:     context.Background(),
		Command: "",
	})
	if err != nil {
		t.Errorf("empty command should return nil, got: %v", err)
	}
	if result != "" {
		t.Errorf("empty command should return empty, got: %q", result)
	}
}
