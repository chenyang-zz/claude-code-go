package shared

import (
	"sync"
	"testing"
)

func TestStoreCreate(t *testing.T) {
	s := NewStore()
	task, err := s.Create("*/5 * * * *", "test prompt", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID == "" {
		t.Error("expected non-empty ID")
	}
	if task.Cron != "*/5 * * * *" {
		t.Errorf("expected cron %q, got %q", "*/5 * * * *", task.Cron)
	}
	if task.Prompt != "test prompt" {
		t.Errorf("expected prompt %q, got %q", "test prompt", task.Prompt)
	}
	if !task.Recurring {
		t.Error("expected recurring to be true")
	}
	if task.Durable {
		t.Error("expected durable to be false")
	}
}

func TestStoreCreateMaxJobs(t *testing.T) {
	s := NewStore()
	// Fill up to MaxJobs.
	for i := 0; i < MaxJobs; i++ {
		_, err := s.Create("* * * * *", "fill", false, false)
		if err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
	}
	// One more should fail.
	_, err := s.Create("* * * * *", "overflow", false, false)
	if err == nil {
		t.Error("expected error when exceeding MaxJobs")
	}
}

func TestStoreDelete(t *testing.T) {
	s := NewStore()
	task, _ := s.Create("0 9 * * *", "daily", true, false)

	if err := s.Delete(task.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Exists(task.ID) {
		t.Error("expected task to be deleted")
	}
}

func TestStoreDeleteNotFound(t *testing.T) {
	s := NewStore()
	err := s.Delete("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestStoreList(t *testing.T) {
	s := NewStore()
	// Empty list.
	if len(s.List()) != 0 {
		t.Error("expected empty list")
	}

	s.Create("*/5 * * * *", "first", true, false)
	s.Create("0 9 * * *", "second", false, false)

	tasks := s.List()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestStoreExists(t *testing.T) {
	s := NewStore()
	if s.Exists("nonexistent") {
		t.Error("expected false for nonexistent task")
	}
	task, _ := s.Create("* * * * *", "test", false, false)
	if !s.Exists(task.ID) {
		t.Error("expected true for created task")
	}
}

func TestStoreCount(t *testing.T) {
	s := NewStore()
	if s.Count() != 0 {
		t.Error("expected 0 count initially")
	}
	s.Create("* * * * *", "a", false, false)
	s.Create("* * * * *", "b", false, false)
	if s.Count() != 2 {
		t.Errorf("expected 2, got %d", s.Count())
	}
}

func TestStoreConcurrency(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	n := 20

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Create("* * * * *", "concurrent", false, false)
		}()
	}
	wg.Wait()

	if s.Count() != n {
		t.Errorf("expected %d tasks, got %d", n, s.Count())
	}
}
