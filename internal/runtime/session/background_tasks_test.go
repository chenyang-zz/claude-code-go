package session

import (
	"testing"

	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
)

// TestBackgroundTaskStoreReplaceAndList verifies the runtime store returns detached snapshots.
func TestBackgroundTaskStoreReplaceAndList(t *testing.T) {
	store := NewBackgroundTaskStore()
	store.Replace([]coresession.BackgroundTaskSnapshot{
		{
			ID:                "task-1",
			Type:              "shell",
			Status:            coresession.BackgroundTaskStatusRunning,
			Summary:           "build watcher",
			ControlsAvailable: false,
		},
	})

	got := store.List()
	if len(got) != 1 {
		t.Fatalf("List() len = %d, want 1", len(got))
	}
	if got[0].Summary != "build watcher" {
		t.Fatalf("List()[0].Summary = %q, want build watcher", got[0].Summary)
	}

	got[0].Summary = "mutated"
	gotAgain := store.List()
	if gotAgain[0].Summary != "build watcher" {
		t.Fatalf("List() should return a detached copy, got summary %q", gotAgain[0].Summary)
	}
}
