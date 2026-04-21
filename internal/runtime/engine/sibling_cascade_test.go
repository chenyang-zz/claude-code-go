package engine

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// --- SiblingCascade unit tests ---

func TestSiblingCascade_TriggerBashError(t *testing.T) {
	parentCtx := context.Background()
	cascade := NewSiblingCascade(parentCtx)

	if cascade.IsErrored() {
		t.Fatal("new cascade should not be errored")
	}
	if cascade.Context().Err() != nil {
		t.Fatal("cascade context should not be cancelled initially")
	}

	cascade.TriggerBashError("Bash(mkdir /foo)")

	if !cascade.IsErrored() {
		t.Fatal("cascade should be errored after trigger")
	}
	if cascade.ErroredToolDesc() != "Bash(mkdir /foo)" {
		t.Fatalf("expected desc 'Bash(mkdir /foo)', got '%s'", cascade.ErroredToolDesc())
	}
	if cascade.Context().Err() == nil {
		t.Fatal("cascade context should be cancelled after trigger")
	}
}

func TestSiblingCascade_DoesNotPropagateToParent(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	cascade := NewSiblingCascade(parentCtx)
	cascade.TriggerBashError("Bash(test)")

	// Cascade context should be cancelled
	if cascade.Context().Err() == nil {
		t.Fatal("cascade context should be cancelled")
	}
	// Parent context should NOT be cancelled
	if parentCtx.Err() != nil {
		t.Fatal("cascade cancel should NOT propagate to parent context")
	}
}

func TestSiblingCascade_ParentCancelPropagatesToCascade(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())
	cascade := NewSiblingCascade(parentCtx)

	parentCancel()

	// Cascade context (derived from parent) should be cancelled
	if cascade.Context().Err() == nil {
		t.Fatal("parent cancel should propagate to cascade context")
	}
	// Cascade should NOT report as errored (not triggered by Bash)
	if cascade.IsErrored() {
		t.Fatal("parent cancel should not set hasErrored")
	}
}

func TestTriggerOnBashError_OnlyBashTriggers(t *testing.T) {
	cascade := NewSiblingCascade(context.Background())

	// Non-Bash tool error should NOT trigger
	triggered := cascade.TriggerOnBashError("Read", map[string]any{"path": "/tmp"}, coretool.Result{}, errors.New("read error"))
	if triggered {
		t.Fatal("non-Bash tool error should not trigger cascade")
	}
	if cascade.IsErrored() {
		t.Fatal("non-Bash tool error should not set hasErrored")
	}

	// Bash tool error should trigger
	triggered = cascade.TriggerOnBashError("Bash", map[string]any{"command": "mkdir /foo"}, coretool.Result{}, errors.New("exit status 1"))
	if !triggered {
		t.Fatal("Bash tool error should trigger cascade")
	}
	if !cascade.IsErrored() {
		t.Fatal("Bash tool error should set hasErrored")
	}
}

func TestTriggerOnBashError_NoErrorResult(t *testing.T) {
	cascade := NewSiblingCascade(context.Background())

	// Bash tool with successful result should NOT trigger
	triggered := cascade.TriggerOnBashError("Bash", map[string]any{"command": "ls"}, coretool.Result{Output: "file.txt"}, nil)
	if triggered {
		t.Fatal("Bash tool with successful result should not trigger cascade")
	}

	// Bash tool with error result (result.Error) should trigger
	triggered = cascade.TriggerOnBashError("Bash", map[string]any{"command": "ls"}, coretool.Result{Error: "permission denied"}, nil)
	if !triggered {
		t.Fatal("Bash tool with error result should trigger cascade")
	}
}

// --- FormatCascadeErrorMessage tests ---

func TestFormatCascadeErrorMessage(t *testing.T) {
	tests := []struct {
		desc     string
		expected string
	}{
		{"Bash(mkdir /foo)", "Cancelled: parallel tool call Bash(mkdir /foo) errored"},
		{"", "Cancelled: parallel tool call errored"},
		{"Bash(rm -rf ...)", "Cancelled: parallel tool call Bash(rm -rf ...) errored"},
	}
	for _, tt := range tests {
		got := FormatCascadeErrorMessage(tt.desc)
		if got != tt.expected {
			t.Errorf("FormatCascadeErrorMessage(%q) = %q, want %q", tt.desc, got, tt.expected)
		}
	}
}

// --- FormatToolDescription tests ---

func TestFormatToolDescription(t *testing.T) {
	// Normal input
	desc := FormatToolDescription("Bash", map[string]any{"command": "mkdir /foo/bar"})
	if desc != "Bash(mkdir /foo/bar)" {
		t.Fatalf("expected 'Bash(mkdir /foo/bar)', got '%s'", desc)
	}

	// Long input gets truncated
	longCmd := strings.Repeat("x", 50)
	desc = FormatToolDescription("Bash", map[string]any{"command": longCmd})
	if !strings.Contains(desc, "...") {
		t.Fatalf("long input should be truncated with '...', got '%s'", desc)
	}
	expectedLen := len("Bash(") + 40 + len("...") + len(")")
	if len(desc) != expectedLen {
		t.Fatalf("truncated description should be %d chars, got %d: '%s'", expectedLen, len(desc), desc)
	}

	// Empty input
	desc = FormatToolDescription("Read", map[string]any{})
	if desc != "Read()" {
		t.Fatalf("expected 'Read()', got '%s'", desc)
	}

	// Non-string values are ignored
	desc = FormatToolDescription("Bash", map[string]any{"timeout": 30, "command": "ls"})
	if desc != "Bash(ls)" {
		t.Fatalf("expected 'Bash(ls)', got '%s'", desc)
	}
}

// --- StreamingToolExecutor cascade integration tests ---

func TestStreamingToolExecutor_BashErrorCancelsSiblings(t *testing.T) {
	fake := newFakeStreamingExecute()
	var bashDone int32
	fake.run = func(ctx context.Context, call coretool.Call) (coretool.Result, error) {
		if call.Name == "Bash" {
			time.Sleep(20 * time.Millisecond)
			atomic.StoreInt32(&bashDone, 1)
			return coretool.Result{Error: "exit status 1"}, nil
		}
		// Other tool: wait for context cancellation or timeout
		select {
		case <-time.After(5 * time.Second):
			return coretool.Result{Output: "should not reach"}, nil
		case <-ctx.Done():
			return coretool.Result{}, ctx.Err()
		}
	}

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		func(ctx context.Context, call coretool.Call, _ chan<- event.Event) (coretool.Result, error) {
			return fake.run(ctx, call)
		},
		func(string) bool { return true }, // all concurrency-safe
		out,
		10,
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "1", Name: "Bash", Input: map[string]any{"command": "false"}})
	exec.AddTool(context.Background(), model.ToolUse{ID: "2", Name: "Read", Input: map[string]any{"path": "/tmp/x"}})

	results := exec.AwaitAll(context.Background())

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	if atomic.LoadInt32(&bashDone) != 1 {
		t.Fatal("bash tool should have completed")
	}

	if !exec.cascade.IsErrored() {
		t.Fatal("cascade should be errored after Bash tool failure")
	}
}

func TestStreamingToolExecutor_NonBashErrorDoesNotCancelSiblings(t *testing.T) {
	fake := newFakeStreamingExecute()
	var read1Done, read2Done int32

	fake.run = func(ctx context.Context, call coretool.Call) (coretool.Result, error) {
		if call.ID == "1" {
			time.Sleep(20 * time.Millisecond)
			atomic.StoreInt32(&read1Done, 1)
			return coretool.Result{Error: "file not found"}, nil
		}
		time.Sleep(50 * time.Millisecond)
		atomic.StoreInt32(&read2Done, 1)
		return coretool.Result{Output: "ok"}, nil
	}

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		func(ctx context.Context, call coretool.Call, _ chan<- event.Event) (coretool.Result, error) {
			return fake.run(ctx, call)
		},
		func(string) bool { return true },
		out,
		10,
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "1", Name: "Read", Input: map[string]any{"path": "/missing"}})
	exec.AddTool(context.Background(), model.ToolUse{ID: "2", Name: "Read", Input: map[string]any{"path": "/exists"}})

	results := exec.AwaitAll(context.Background())

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	if atomic.LoadInt32(&read1Done) != 1 || atomic.LoadInt32(&read2Done) != 1 {
		t.Fatal("both tools should complete when non-Bash tool errors")
	}

	if exec.cascade.IsErrored() {
		t.Fatal("cascade should NOT be triggered by non-Bash tool error")
	}
}

func TestStreamingToolExecutor_CascadeProducesSyntheticError(t *testing.T) {
	fake := newFakeStreamingExecute()

	fake.run = func(ctx context.Context, call coretool.Call) (coretool.Result, error) {
		if call.Name == "Bash" {
			return coretool.Result{Error: "exit status 1"}, nil
		}
		select {
		case <-time.After(5 * time.Second):
			return coretool.Result{Output: "should not reach"}, nil
		case <-ctx.Done():
			return coretool.Result{}, ctx.Err()
		}
	}

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		func(ctx context.Context, call coretool.Call, _ chan<- event.Event) (coretool.Result, error) {
			return fake.run(ctx, call)
		},
		func(string) bool { return true },
		out,
		10,
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "1", Name: "Bash", Input: map[string]any{"command": "false"}})
	exec.AddTool(context.Background(), model.ToolUse{ID: "2", Name: "Read", Input: map[string]any{"path": "/tmp/x"}})

	exec.AwaitAll(context.Background())

	// Check the Read tool (index 2 in tools slice) has synthetic cascade error
	msg := exec.BuildToolResultMessage()
	foundCascadeError := false
	for _, part := range msg.Content {
		if part.Type == "tool_result" && part.ToolUseID == "2" {
			if !strings.Contains(part.Text, "Cancelled: parallel tool call") {
				t.Errorf("expected cascade error message in Read tool result, got: %s", part.Text)
			}
			if !part.IsError {
				t.Error("cascade cancelled tool result should be marked as error")
			}
			foundCascadeError = true
		}
	}
	if !foundCascadeError {
		t.Fatal("expected to find tool_result for Read tool with cascade error")
	}
}

func TestStreamingToolExecutor_CascadeDoesNotAffectParentContext(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	fake := newFakeStreamingExecute()
	fake.run = func(ctx context.Context, call coretool.Call) (coretool.Result, error) {
		if call.Name == "Bash" {
			return coretool.Result{Error: "exit status 1"}, nil
		}
		select {
		case <-time.After(5 * time.Second):
			return coretool.Result{Output: "ok"}, nil
		case <-ctx.Done():
			return coretool.Result{}, ctx.Err()
		}
	}

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		parentCtx,
		func(ctx context.Context, call coretool.Call, _ chan<- event.Event) (coretool.Result, error) {
			return fake.run(ctx, call)
		},
		func(string) bool { return true },
		out,
		10,
	)

	exec.AddTool(parentCtx, model.ToolUse{ID: "1", Name: "Bash", Input: map[string]any{"command": "false"}})
	exec.AddTool(parentCtx, model.ToolUse{ID: "2", Name: "Read", Input: map[string]any{"path": "/tmp/x"}})

	exec.AwaitAll(parentCtx)

	if parentCtx.Err() != nil {
		t.Fatal("parent context should NOT be cancelled by sibling cascade")
	}
}

func TestStreamingToolExecutor_QueuedToolsGetSyntheticError(t *testing.T) {
	fake := newFakeStreamingExecute()

	fake.run = func(ctx context.Context, call coretool.Call) (coretool.Result, error) {
		if call.Name == "Bash" {
			time.Sleep(50 * time.Millisecond)
			return coretool.Result{Error: "exit status 1"}, nil
		}
		// Third tool should never execute because cascade triggers before it starts
		return coretool.Result{Output: "should not execute"}, nil
	}

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		func(ctx context.Context, call coretool.Call, _ chan<- event.Event) (coretool.Result, error) {
			return fake.run(ctx, call)
		},
		func(string) bool { return true },
		out,
		1, // Only 1 concurrent tool, so third tool stays queued
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "1", Name: "Bash", Input: map[string]any{"command": "false"}})
	exec.AddTool(context.Background(), model.ToolUse{ID: "2", Name: "Read", Input: map[string]any{"path": "/a"}})
	exec.AddTool(context.Background(), model.ToolUse{ID: "3", Name: "Glob", Input: map[string]any{"pattern": "*.go"}})

	exec.AwaitAll(context.Background())

	// All tools should be completed (including queued ones that got synthetic errors)
	for i := range exec.tools {
		if exec.tools[i].status != streamingToolCompleted && exec.tools[i].status != streamingToolYielded {
			t.Errorf("tool %d (%s) should be completed, got status %s", i, exec.tools[i].toolUse.Name, exec.tools[i].status)
		}
	}

	// Third tool should have synthetic error
	if exec.tools[2].result.Error == "" {
		t.Error("queued tool should have synthetic error message")
	}
	if !strings.Contains(exec.tools[2].result.Error, "Cancelled: parallel tool call") {
		t.Errorf("queued tool error should contain cascade message, got: %s", exec.tools[2].result.Error)
	}
}

// --- executeToolBatch cascade integration tests ---

func TestExecuteToolBatch_BashErrorCancelsSiblings(t *testing.T) {
	rt := &Runtime{
		Executor: &fakeToolExecutor{
			run: func(ctx context.Context, call coretool.Call) (coretool.Result, error) {
				if call.Name == "Bash" {
					return coretool.Result{Error: "exit status 1"}, nil
				}
				select {
				case <-time.After(5 * time.Second):
					return coretool.Result{Output: "should not reach"}, nil
				case <-ctx.Done():
					return coretool.Result{}, ctx.Err()
				}
			},
			safe: map[string]bool{"Bash": true, "Read": true, "Glob": true},
		},
		MaxConcurrentToolCalls: 10,
	}

	batch := toolExecutionBatch{
		concurrencySafe: true,
		toolUses: []model.ToolUse{
			{ID: "1", Name: "Bash", Input: map[string]any{"command": "false"}},
			{ID: "2", Name: "Read", Input: map[string]any{"path": "/tmp/x"}},
			{ID: "3", Name: "Glob", Input: map[string]any{"pattern": "*.go"}},
		},
	}

	outcomes := rt.executeToolBatch(context.Background(), batch, make(chan event.Event, 10))

	if len(outcomes) != 3 {
		t.Fatalf("expected 3 outcomes, got %d", len(outcomes))
	}

	// Bash tool should have real error
	if outcomes[0].invokeErr != nil || outcomes[0].result.Error != "exit status 1" {
		t.Errorf("Bash tool should have real error, got result.Error=%q invokeErr=%v", outcomes[0].result.Error, outcomes[0].invokeErr)
	}

	// Sibling tools should have synthetic cascade errors
	for i := 1; i < 3; i++ {
		if outcomes[i].invokeErr != nil {
			t.Errorf("outcome %d should have nil invokeErr (replaced by synthetic), got %v", i, outcomes[i].invokeErr)
		}
		if !strings.Contains(outcomes[i].result.Error, "Cancelled: parallel tool call") {
			t.Errorf("outcome %d should have cascade error, got: %s", i, outcomes[i].result.Error)
		}
	}
}

func TestExecuteToolBatch_NonBashErrorDoesNotCancel(t *testing.T) {
	var execCount int64
	rt := &Runtime{
		Executor: &fakeToolExecutor{
			run: func(ctx context.Context, call coretool.Call) (coretool.Result, error) {
				atomic.AddInt64(&execCount, 1)
				if call.ID == "1" {
					return coretool.Result{Error: "file not found"}, nil
				}
				return coretool.Result{Output: "ok"}, nil
			},
			safe: map[string]bool{"Read": true},
		},
		MaxConcurrentToolCalls: 10,
	}

	batch := toolExecutionBatch{
		concurrencySafe: true,
		toolUses: []model.ToolUse{
			{ID: "1", Name: "Read", Input: map[string]any{"path": "/missing"}},
			{ID: "2", Name: "Read", Input: map[string]any{"path": "/exists"}},
		},
	}

	outcomes := rt.executeToolBatch(context.Background(), batch, make(chan event.Event, 10))

	if len(outcomes) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(outcomes))
	}

	if atomic.LoadInt64(&execCount) != 2 {
		t.Fatal("both tools should have executed when non-Bash tool errors")
	}

	if outcomes[0].result.Error != "file not found" {
		t.Errorf("first tool should have 'file not found' error, got: %s", outcomes[0].result.Error)
	}

	if outcomes[1].result.Output != "ok" {
		t.Errorf("second tool should have 'ok' output, got: %s", outcomes[1].result.Output)
	}
}
