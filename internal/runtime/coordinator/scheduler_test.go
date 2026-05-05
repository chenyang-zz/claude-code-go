package coordinator

import (
	"context"
	"fmt"
	"testing"
)

// mockRunner is a test double for AgentRunner.
type mockRunner struct {
	output AgentOutput
	err    error
}

func (m *mockRunner) Run(_ context.Context, _ AgentInput) (AgentOutput, error) {
	return m.output, m.err
}

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

func TestScheduleAsyncSuccess(t *testing.T) {
	runner := &mockRunner{
		output: AgentOutput{Content: "async result", TotalTokens: 100},
	}
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(runner, cfg)

	input := AgentInput{
		Description:  "async task",
		Prompt:       "do async work",
		SubagentType: "worker",
	}

	w, ch := s.ScheduleAsync(context.Background(), input)
	if w == nil {
		t.Fatal("expected non-nil worker")
	}
	if w.State != WorkerStateCreated && w.State != WorkerStateRunning {
		t.Errorf("expected created or running state, got %s", w.State)
	}

	result := <-ch
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Output.Content != "async result" {
		t.Errorf("expected content 'async result', got %q", result.Output.Content)
	}
	if result.Worker == nil {
		t.Error("expected non-nil worker in result")
	}

	// Worker should be removed from tracking after completion
	if s.GetWorker(w.ID) != nil {
		t.Error("expected worker to be removed from tracking after completion")
	}
}

func TestScheduleAsyncRunnerError(t *testing.T) {
	runner := &mockRunner{
		err: fmt.Errorf("execution failed"),
	}
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(runner, cfg)

	input := AgentInput{
		Description:  "failing task",
		Prompt:       "fail",
		SubagentType: "worker",
	}

	w, ch := s.ScheduleAsync(context.Background(), input)
	if w == nil {
		t.Fatal("expected non-nil worker")
	}

	result := <-ch
	if result.Error == nil {
		t.Error("expected error from failing runner")
	}

	// Worker should still be removed from tracking
	if s.GetWorker(w.ID) != nil {
		t.Error("expected worker to be removed from tracking after failure")
	}
}

func TestScheduleAsyncNilRunner(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(nil, cfg)

	input := AgentInput{
		Description:  "nil runner task",
		Prompt:       "no runner",
		SubagentType: "worker",
	}

	w, ch := s.ScheduleAsync(context.Background(), input)
	if w != nil {
		t.Error("expected nil worker for nil runner")
	}

	result := <-ch
	if result.Error == nil {
		t.Error("expected error for nil runner")
	}
}

func TestScheduleAsyncMaxConcurrent(t *testing.T) {
	runner := &mockRunner{output: AgentOutput{Content: "done"}}
	cfg := SchedulerConfig{
		MaxConcurrentWorkers: 1,
		DefaultMaxTurns:      8,
	}
	s := NewScheduler(runner, cfg)

	input := AgentInput{
		Description:  "task",
		Prompt:       "work",
		SubagentType: "worker",
	}

	// First async worker should succeed
	w1, ch1 := s.ScheduleAsync(context.Background(), input)
	if w1 == nil {
		t.Fatal("expected non-nil worker")
	}

	// Wait for first to complete
	<-ch1

	// Second should also succeed (first is done and removed)
	w2, ch2 := s.ScheduleAsync(context.Background(), input)
	if w2 == nil {
		t.Fatal("expected non-nil worker for second call")
	}
	<-ch2
}

func TestCollectResultsSuccess(t *testing.T) {
	runner1 := &mockRunner{output: AgentOutput{Content: "result1"}}
	runner2 := &mockRunner{output: AgentOutput{Content: "result2"}}
	cfg := DefaultSchedulerConfig()

	s1 := NewScheduler(runner1, cfg)
	s2 := NewScheduler(runner2, cfg)

	input := AgentInput{Description: "task", Prompt: "work", SubagentType: "worker"}

	_, ch1 := s1.ScheduleAsync(context.Background(), input)
	_, ch2 := s2.ScheduleAsync(context.Background(), input)

	results := CollectResults(context.Background(), []<-chan WorkerResult{ch1, ch2})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	contents := map[string]bool{}
	for _, r := range results {
		if r.Error != nil {
			t.Errorf("unexpected error: %v", r.Error)
		}
		contents[r.Output.Content] = true
	}
	if !contents["result1"] || !contents["result2"] {
		t.Errorf("expected both results, got %v", contents)
	}
}

func TestCollectResultsContextCancel(t *testing.T) {
	runner := &mockRunner{output: AgentOutput{Content: "done"}}
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(runner, cfg)

	input := AgentInput{Description: "task", Prompt: "work", SubagentType: "worker"}

	_, ch := s.ScheduleAsync(context.Background(), input)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	results := CollectResults(ctx, []<-chan WorkerResult{ch})
	// Should return at least one result (either from channel or context error)
	if len(results) == 0 {
		t.Error("expected at least one result")
	}
}

func TestCollectResultsEmpty(t *testing.T) {
	results := CollectResults(context.Background(), []<-chan WorkerResult{})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestScheduleAsyncCleansUpOnCompletion(t *testing.T) {
	runner := &mockRunner{output: AgentOutput{Content: "done"}}
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(runner, cfg)

	input := AgentInput{Description: "task", Prompt: "work", SubagentType: "worker"}

	w, ch := s.ScheduleAsync(context.Background(), input)
	if s.ActiveWorkers() != 1 {
		t.Fatalf("expected 1 active worker, got %d", s.ActiveWorkers())
	}

	<-ch

	// Worker should be removed from tracking
	if s.ActiveWorkers() != 0 {
		t.Errorf("expected 0 active workers after completion, got %d", s.ActiveWorkers())
	}
	if s.GetWorker(w.ID) != nil {
		t.Error("expected worker to be removed from tracking")
	}
}
