package coordinator

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestFormatTaskNotificationCompleted(t *testing.T) {
	w := &Worker{
		ID:    "worker-abc123",
		State: WorkerStateCompleted,
		Input: AgentInput{Description: "Investigate auth bug"},
		Output: AgentOutput{
			Content:           "Found null pointer in validate.ts:42",
			TotalTokens:       500,
			TotalToolUseCount: 3,
			TotalDurationMs:   1500,
		},
	}
	result := WorkerResult{Worker: w, Output: w.Output}

	notif := FormatTaskNotification(result)

	if !strings.Contains(notif, "<task-notification>") {
		t.Error("expected <task-notification> tag")
	}
	if !strings.Contains(notif, "<task-id>worker-abc123</task-id>") {
		t.Errorf("expected task-id, got: %s", notif)
	}
	if !strings.Contains(notif, "<status>completed</status>") {
		t.Errorf("expected completed status, got: %s", notif)
	}
	if !strings.Contains(notif, "Investigate auth bug") {
		t.Errorf("expected description in summary, got: %s", notif)
	}
	if !strings.Contains(notif, "Found null pointer in validate.ts:42") {
		t.Errorf("expected result content, got: %s", notif)
	}
	if !strings.Contains(notif, "<total_tokens>500</total_tokens>") {
		t.Errorf("expected total_tokens, got: %s", notif)
	}
	if !strings.Contains(notif, "<tool_uses>3</tool_uses>") {
		t.Errorf("expected tool_uses, got: %s", notif)
	}
	if !strings.Contains(notif, "<duration_ms>1500</duration_ms>") {
		t.Errorf("expected duration_ms, got: %s", notif)
	}
	if !strings.Contains(notif, "</task-notification>") {
		t.Error("expected closing </task-notification> tag")
	}
}

func TestFormatTaskNotificationFailed(t *testing.T) {
	w := &Worker{
		ID:    "worker-fail1",
		State: WorkerStateFailed,
		Input: AgentInput{Description: "Fix build error"},
		Error: fmt.Errorf("compilation failed: undefined variable x"),
	}
	result := WorkerResult{Worker: w, Error: w.Error}

	notif := FormatTaskNotification(result)

	if !strings.Contains(notif, "<status>failed</status>") {
		t.Errorf("expected failed status, got: %s", notif)
	}
	if !strings.Contains(notif, "compilation failed") {
		t.Errorf("expected error in result, got: %s", notif)
	}
	// Failed notifications should NOT have usage section
	if strings.Contains(notif, "<usage>") {
		t.Errorf("unexpected usage section in failed notification: %s", notif)
	}
}

func TestFormatTaskNotificationStopped(t *testing.T) {
	w := &Worker{
		ID:    "worker-stop1",
		State: WorkerStateStopped,
		Input: AgentInput{Description: "Long running task"},
	}
	result := WorkerResult{Worker: w}

	notif := FormatTaskNotification(result)

	if !strings.Contains(notif, "<status>killed</status>") {
		t.Errorf("expected killed status for stopped worker, got: %s", notif)
	}
	if !strings.Contains(notif, "was stopped") {
		t.Errorf("expected 'was stopped' in summary, got: %s", notif)
	}
}

func TestFormatTaskNotificationRunning(t *testing.T) {
	w := &Worker{
		ID:    "worker-run1",
		State: WorkerStateRunning,
		Input: AgentInput{Description: "Active task"},
	}
	result := WorkerResult{Worker: w}

	notif := FormatTaskNotification(result)

	if !strings.Contains(notif, "<status>running</status>") {
		t.Errorf("expected running status, got: %s", notif)
	}
	if !strings.Contains(notif, "is running") {
		t.Errorf("expected 'is running' in summary, got: %s", notif)
	}
}

func TestFormatTaskNotificationPending(t *testing.T) {
	w := &Worker{
		ID:    "worker-pend1",
		State: WorkerStateCreated,
		Input: AgentInput{Description: "Queued task"},
	}
	result := WorkerResult{Worker: w}

	notif := FormatTaskNotification(result)

	if !strings.Contains(notif, "<status>pending</status>") {
		t.Errorf("expected pending status, got: %s", notif)
	}
}

func TestFormatTaskNotificationNilWorker(t *testing.T) {
	result := WorkerResult{Error: fmt.Errorf("worker creation failed")}

	notif := FormatTaskNotification(result)

	if !strings.Contains(notif, "<status>failed</status>") {
		t.Errorf("expected failed status for nil worker, got: %s", notif)
	}
	if !strings.Contains(notif, "worker creation failed") {
		t.Errorf("expected error message, got: %s", notif)
	}
}

func TestFormatTaskNotificationFromWorker(t *testing.T) {
	w := &Worker{
		ID:    "worker-direct1",
		State: WorkerStateCompleted,
		Input: AgentInput{Description: "Test task"},
		Output: AgentOutput{
			Content:     "Task done",
			TotalTokens: 100,
		},
	}

	notif := FormatTaskNotificationFromWorker(w)

	if !strings.Contains(notif, "<task-id>worker-direct1</task-id>") {
		t.Errorf("expected task-id, got: %s", notif)
	}
	if !strings.Contains(notif, "<status>completed</status>") {
		t.Errorf("expected completed status, got: %s", notif)
	}
	if !strings.Contains(notif, "Task done") {
		t.Errorf("expected result content, got: %s", notif)
	}
}

func TestFormatTaskNotificationFromNilWorker(t *testing.T) {
	notif := FormatTaskNotificationFromWorker(nil)

	if !strings.Contains(notif, "<status>failed</status>") {
		t.Errorf("expected failed status for nil worker, got: %s", notif)
	}
}

func TestDispatchAsyncSuccess(t *testing.T) {
	runner := &mockRunner{
		output: AgentOutput{Content: "dispatch result", TotalTokens: 200},
	}
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(runner, cfg)

	input := AgentInput{
		Description:  "dispatch test",
		Prompt:       "do work",
		SubagentType: "worker",
	}

	w, ch := DispatchAsync(context.Background(), s, input)
	if w == nil {
		t.Fatal("expected non-nil worker")
	}

	result := <-ch
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Worker == nil {
		t.Error("expected non-nil worker in result")
	}
	if !strings.Contains(result.Notification, "<task-notification>") {
		t.Errorf("expected notification XML, got: %s", result.Notification)
	}
	if !strings.Contains(result.Notification, "dispatch result") {
		t.Errorf("expected result content in notification, got: %s", result.Notification)
	}
	if !strings.Contains(result.Notification, "<status>completed</status>") {
		t.Errorf("expected completed status, got: %s", result.Notification)
	}
}

func TestDispatchAsyncFailure(t *testing.T) {
	runner := &mockRunner{
		err: fmt.Errorf("dispatch failed"),
	}
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(runner, cfg)

	input := AgentInput{
		Description:  "failing dispatch",
		Prompt:       "fail",
		SubagentType: "worker",
	}

	w, ch := DispatchAsync(context.Background(), s, input)
	if w == nil {
		t.Fatal("expected non-nil worker")
	}

	result := <-ch
	if result.Error == nil {
		t.Error("expected error from failing dispatch")
	}
	if !strings.Contains(result.Notification, "<status>failed</status>") {
		t.Errorf("expected failed status in notification, got: %s", result.Notification)
	}
}

func TestDispatchAsyncNilRunner(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	s := NewScheduler(nil, cfg)

	input := AgentInput{
		Description:  "nil runner dispatch",
		Prompt:       "no runner",
		SubagentType: "worker",
	}

	w, ch := DispatchAsync(context.Background(), s, input)
	if w != nil {
		t.Error("expected nil worker for nil runner")
	}

	result := <-ch
	if result.Error == nil {
		t.Error("expected error for nil runner")
	}
}

func TestWorkerStateToStatus(t *testing.T) {
	tests := []struct {
		state    WorkerState
		expected string
	}{
		{WorkerStateCreated, "pending"},
		{WorkerStateRunning, "running"},
		{WorkerStateCompleted, "completed"},
		{WorkerStateFailed, "failed"},
		{WorkerStateStopped, "killed"},
	}

	for _, tt := range tests {
		got := workerStateToStatus(tt.state)
		if got != tt.expected {
			t.Errorf("workerStateToStatus(%v) = %q, want %q", tt.state, got, tt.expected)
		}
	}
}
