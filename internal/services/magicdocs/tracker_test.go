package magicdocs_test

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/services/magicdocs"
)

func TestTracker_Register(t *testing.T) {
	magicdocs.ClearTrackedMagicDocs()
	magicdocs.RegisterMagicDoc("/path/to/doc.md")
	if !magicdocs.HasTrackedDocs() {
		t.Error("expected HasTrackedDocs() to return true after registering")
	}
	docs := magicdocs.TrackedDocs()
	found := false
	for _, d := range docs {
		if d.Path == "/path/to/doc.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TrackedDocs() to contain /path/to/doc.md, got %+v", docs)
	}
}

func TestTracker_RegisterIdempotent(t *testing.T) {
	magicdocs.ClearTrackedMagicDocs()
	magicdocs.RegisterMagicDoc("/path/to/doc.md")
	magicdocs.RegisterMagicDoc("/path/to/doc.md")
	docs := magicdocs.TrackedDocs()
	count := 0
	for _, d := range docs {
		if d.Path == "/path/to/doc.md" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 entry for /path/to/doc.md after duplicate registration, got %d", count)
	}
	if len(docs) != 1 {
		t.Errorf("expected TrackedDocs() to have 1 entry, got %d", len(docs))
	}
}

func TestTracker_Unregister(t *testing.T) {
	magicdocs.ClearTrackedMagicDocs()
	magicdocs.RegisterMagicDoc("/path/to/doc.md")
	magicdocs.UnregisterMagicDoc("/path/to/doc.md")
	if magicdocs.HasTrackedDocs() {
		t.Error("expected HasTrackedDocs() to return false after unregistering the only entry")
	}
	docs := magicdocs.TrackedDocs()
	if len(docs) != 0 {
		t.Errorf("expected empty TrackedDocs() after unregistering, got %+v", docs)
	}
}

func TestTracker_Clear(t *testing.T) {
	magicdocs.RegisterMagicDoc("/path/to/a.md")
	magicdocs.RegisterMagicDoc("/path/to/b.md")
	magicdocs.RegisterMagicDoc("/path/to/c.md")
	magicdocs.ClearTrackedMagicDocs()
	if magicdocs.HasTrackedDocs() {
		t.Error("expected HasTrackedDocs() to return false after clearing")
	}
	docs := magicdocs.TrackedDocs()
	if len(docs) != 0 {
		t.Errorf("expected empty TrackedDocs() after clearing, got %d entries", len(docs))
	}
}

func TestTracker_EmptyTracker(t *testing.T) {
	magicdocs.ClearTrackedMagicDocs()
	if magicdocs.HasTrackedDocs() {
		t.Error("expected HasTrackedDocs() to return false for empty tracker")
	}
	docs := magicdocs.TrackedDocs()
	if len(docs) != 0 {
		t.Errorf("expected empty slice from TrackedDocs(), got %d entries", len(docs))
	}
}
