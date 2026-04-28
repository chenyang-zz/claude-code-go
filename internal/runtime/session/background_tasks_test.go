package session

import (
	"testing"

	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
)

type recordingStopper struct {
	stopped bool
}

func (s *recordingStopper) Stop() error {
	s.stopped = true
	return nil
}

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

// TestBackgroundTaskStoreRegisterUpdateAndStop verifies the runtime store supports lifecycle registration, updates, and stop removal.
func TestBackgroundTaskStoreRegisterUpdateAndStop(t *testing.T) {
	store := NewBackgroundTaskStore()
	stopper := &recordingStopper{}
	store.Register(coresession.BackgroundTaskSnapshot{
		ID:                "task-1",
		Type:              "bash",
		Status:            coresession.BackgroundTaskStatusRunning,
		Summary:           "npm run dev",
		ControlsAvailable: true,
	}, stopper)

	got := store.List()
	if len(got) != 1 {
		t.Fatalf("List() len = %d, want 1", len(got))
	}
	if got[0].ID != "task-1" {
		t.Fatalf("List()[0].ID = %q, want task-1", got[0].ID)
	}

	updated := coresession.BackgroundTaskSnapshot{
		ID:                "task-1",
		Type:              "bash",
		Status:            coresession.BackgroundTaskStatusPending,
		Summary:           "starting dev server",
		ControlsAvailable: true,
	}
	if ok := store.Update(updated); !ok {
		t.Fatal("Update() ok = false, want true")
	}

	snapshot, ok := store.Get("task-1")
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	if snapshot.Status != coresession.BackgroundTaskStatusPending {
		t.Fatalf("Get() status = %q, want %q", snapshot.Status, coresession.BackgroundTaskStatusPending)
	}

	stopped, err := store.Stop("task-1")
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if stopped.Status != coresession.BackgroundTaskStatusStopped {
		t.Fatalf("Stop() status = %q, want %q", stopped.Status, coresession.BackgroundTaskStatusStopped)
	}
	if !stopper.stopped {
		t.Fatal("Stop() did not call stopper")
	}
	gotAfterStop := store.List()
	if len(gotAfterStop) != 1 {
		t.Fatalf("List() len after Stop = %d, want 1", len(gotAfterStop))
	}
	if gotAfterStop[0].Status != coresession.BackgroundTaskStatusStopped {
		t.Fatalf("List()[0].Status after Stop = %q, want %q", gotAfterStop[0].Status, coresession.BackgroundTaskStatusStopped)
	}
	if gotAfterStop[0].ControlsAvailable {
		t.Fatal("List()[0].ControlsAvailable after Stop = true, want false")
	}
}

// TestBackgroundTaskStoreStopTwiceRejected verifies a stopped task cannot be stopped repeatedly.
func TestBackgroundTaskStoreStopTwiceRejected(t *testing.T) {
	store := NewBackgroundTaskStore()
	store.Register(coresession.BackgroundTaskSnapshot{
		ID:                "task-1",
		Type:              "bash",
		Status:            coresession.BackgroundTaskStatusRunning,
		Summary:           "npm run dev",
		ControlsAvailable: true,
	}, &recordingStopper{})

	if _, err := store.Stop("task-1"); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}

	if _, err := store.Stop("task-1"); err == nil {
		t.Fatal("second Stop() error = nil, want cannot-be-stopped error")
	}
}
