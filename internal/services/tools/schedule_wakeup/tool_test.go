package schedule_wakeup

import (
	"context"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/repl"
)

func newTestTool() *Tool {
	return NewTool(repl.NewWakeupScheduler())
}

func makeCall(delaySeconds int, reason, prompt string) coretool.Call {
	input := map[string]any{
		"delaySeconds": float64(delaySeconds),
		"reason":       reason,
		"prompt":       prompt,
	}
	return coretool.Call{Input: input}
}

func TestScheduleWakeupTool_Name(t *testing.T) {
	tl := newTestTool()
	if tl.Name() != Name {
		t.Errorf("expected Name %q, got %q", Name, tl.Name())
	}
}

func TestScheduleWakeupTool_Description(t *testing.T) {
	tl := newTestTool()
	if tl.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestScheduleWakeupTool_InputSchema(t *testing.T) {
	tl := newTestTool()
	schema := tl.InputSchema()
	if _, ok := schema.Properties["delaySeconds"]; !ok {
		t.Error("schema missing delaySeconds property")
	}
	if _, ok := schema.Properties["reason"]; !ok {
		t.Error("schema missing reason property")
	}
	if _, ok := schema.Properties["prompt"]; !ok {
		t.Error("schema missing prompt property")
	}
	if schema.Properties["delaySeconds"].Required != true {
		t.Error("delaySeconds should be required")
	}
	if schema.Properties["prompt"].Required != true {
		t.Error("prompt should be required")
	}
}

func TestScheduleWakeupTool_IsReadOnly(t *testing.T) {
	tl := newTestTool()
	if tl.IsReadOnly() {
		t.Error("ScheduleWakeup should not be read-only (it schedules a future action)")
	}
}

func TestScheduleWakeupTool_Invoke(t *testing.T) {
	tl := newTestTool()
	defer tl.scheduler.Stop()

	call := makeCall(120, "testing wakeup", "check the deploy")
	result, err := tl.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke returned result error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}

	// Verify the scheduler has a pending wakeup.
	pending := tl.scheduler.Pending()
	if pending == nil {
		t.Fatal("expected pending wakeup after Invoke")
	}
	if pending.Reason != "testing wakeup" {
		t.Errorf("expected reason %q, got %q", "testing wakeup", pending.Reason)
	}
	if pending.Prompt != "check the deploy" {
		t.Errorf("expected prompt %q, got %q", "check the deploy", pending.Prompt)
	}
}

func TestScheduleWakeupTool_Invoke_EmptyPrompt(t *testing.T) {
	tl := newTestTool()
	defer tl.scheduler.Stop()

	call := makeCall(60, "test", "")
	result, err := tl.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for empty prompt")
	}
}

func TestScheduleWakeupTool_Invoke_ClampDelay(t *testing.T) {
	tl := newTestTool()
	defer tl.scheduler.Stop()

	// Below minimum — should clamp to 60
	call := makeCall(10, "clamp test", "test")
	result, err := tl.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	pending := tl.scheduler.Pending()
	if pending == nil {
		t.Fatal("expected pending wakeup")
	}
	if pending.DelaySeconds != repl.MinWakeupDelaySeconds {
		t.Errorf("expected delay clamped to %d, got %d", repl.MinWakeupDelaySeconds, pending.DelaySeconds)
	}
}
