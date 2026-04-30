package cron

import (
	"sync"
	"testing"
	"time"
)

func TestSchedulerLifecycle(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduler(SchedulerOptions{
		ProjectRoot: dir,
		SessionID:   "test-session",
	})

	s.Start()
	// Give it a tick to attempt lock acquisition.
	time.Sleep(50 * time.Millisecond)

	s.Stop()
	// Should be able to start again after stopping.
	s.Start()
	time.Sleep(50 * time.Millisecond)
	s.Stop()
}

func TestSchedulerDoubleStart(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduler(SchedulerOptions{
		ProjectRoot: dir,
		SessionID:   "test-session",
	})

	s.Start()
	time.Sleep(50 * time.Millisecond)
	// Double start should be a no-op.
	s.Start()
	time.Sleep(50 * time.Millisecond)
	s.Stop()
}

func TestSchedulerGetNextFireTimeEmpty(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduler(SchedulerOptions{
		ProjectRoot: dir,
		SessionID:   "test-session",
	})

	nt := s.GetNextFireTime()
	if nt != nil {
		t.Errorf("expected nil for empty scheduler, got %v", nt)
	}
}

func TestSchedulerOnFire(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	var fired []string

	s := NewScheduler(SchedulerOptions{
		ProjectRoot: dir,
		SessionID:   "test-fire-session",
		OnFire: func(prompt string) {
			mu.Lock()
			fired = append(fired, prompt)
			mu.Unlock()
		},
	})

	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	// No tasks in the file, so nothing should fire.
	mu.Lock()
	n := len(fired)
	mu.Unlock()
	if n != 0 {
		t.Errorf("expected 0 firings, got %d", n)
	}
}

func TestSchedulerStopBeforeStart(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduler(SchedulerOptions{
		ProjectRoot: dir,
	})

	// Stop before start should be safe.
	s.Stop()
	s.Stop()

	// Start then double stop.
	s.Start()
	time.Sleep(50 * time.Millisecond)
	s.Stop()
	s.Stop() // double stop should be safe
}

func TestSchedulerConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduler(SchedulerOptions{
		ProjectRoot: dir,
		SessionID:   "concurrent-test",
	})

	s.Start()
	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.GetNextFireTime()
		}()
	}
	wg.Wait()

	s.Stop()
}
