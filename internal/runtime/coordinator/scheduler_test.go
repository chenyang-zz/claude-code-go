package coordinator

import (
	"context"
	"testing"
)

func TestDefaultSchedulerConfig(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	if cfg.MaxConcurrentWorkers != 0 {
		t.Errorf("expected MaxConcurrentWorkers=0, got %d", cfg.MaxConcurrentWorkers)
	}
	if cfg.DefaultMaxTurns != 8 {
		t.Errorf("expected DefaultMaxTurns=8, got %d", cfg.DefaultMaxTurns)
	}
}

func TestNewScheduler(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(nil, cfg)
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
	if s.Runner != nil {
		t.Error("expected nil runner")
	}
	if s.workers == nil {
		t.Error("expected non-nil workers map")
	}
	if len(s.workers) != 0 {
		t.Errorf("expected empty workers map, got %d", len(s.workers))
	}
}

func TestSchedulerCreateWorker(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(nil, cfg)

	input := AgentInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "worker",
	}

	w, err := s.CreateWorker(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil worker")
	}
	if w.State != WorkerStateCreated {
		t.Errorf("expected state %s, got %s", WorkerStateCreated, w.State)
	}
	if s.ActiveWorkers() != 1 {
		t.Errorf("expected 1 active worker, got %d", s.ActiveWorkers())
	}
}

func TestSchedulerGetWorker(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(nil, cfg)

	input := AgentInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "worker",
	}

	w, _ := s.CreateWorker(input)
	got := s.GetWorker(w.ID)
	if got == nil {
		t.Fatal("expected non-nil worker")
	}
	if got.ID != w.ID {
		t.Errorf("expected worker ID %s, got %s", w.ID, got.ID)
	}

	// Test non-existent worker
	if s.GetWorker("nonexistent") != nil {
		t.Error("expected nil for non-existent worker")
	}
}

func TestSchedulerRemoveWorker(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(nil, cfg)

	input := AgentInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "worker",
	}

	w, _ := s.CreateWorker(input)
	if s.ActiveWorkers() != 1 {
		t.Fatalf("expected 1 active worker, got %d", s.ActiveWorkers())
	}

	s.RemoveWorker(w.ID)
	if s.ActiveWorkers() != 0 {
		t.Errorf("expected 0 active workers, got %d", s.ActiveWorkers())
	}
}

func TestSchedulerMaxConcurrentWorkers(t *testing.T) {
	cfg := SchedulerConfig{
		MaxConcurrentWorkers: 2,
		DefaultMaxTurns:      8,
	}
	s := NewScheduler(nil, cfg)

	input := AgentInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "worker",
	}

	// Create first worker and set it to running
	w1, err := s.CreateWorker(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w1.State = WorkerStateRunning

	// Create second worker and set it to running
	w2, err := s.CreateWorker(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w2.State = WorkerStateRunning

	// Try to create third worker (should fail because 2 are running)
	_, err = s.CreateWorker(input)
	if err == nil {
		t.Error("expected error for max concurrent workers")
	}

	// Set first worker to completed; creating another should now succeed
	w1.State = WorkerStateCompleted
	_, err = s.CreateWorker(input)
	if err != nil {
		t.Errorf("expected no error after worker completed, got: %v", err)
	}
}

func TestSchedulerScheduleNoRunner(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(nil, cfg)

	input := AgentInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "worker",
	}

	_, err := s.Schedule(context.Background(), input)
	if err == nil {
		t.Error("expected error for nil runner")
	}
}

func TestSchedulerStopWorkerNotFound(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(nil, cfg)

	err := s.StopWorker("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent worker")
	}
}
