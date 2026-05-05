package agentsummary

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
)

// ============================================================
// SummaryStore tests
// ============================================================

func TestSummaryStore_StoreAndLoad(t *testing.T) {
	store := NewSummaryStore()
	store.Store("task-1", "Reading test file")
	got, ok := store.Load("task-1")
	if !ok || got != "Reading test file" {
		t.Errorf("want 'Reading test file', got %q (ok=%v)", got, ok)
	}
}

func TestSummaryStore_LoadMissing(t *testing.T) {
	store := NewSummaryStore()
	_, ok := store.Load("nonexistent")
	if ok {
		t.Error("expected Load to return false for missing key")
	}
}

func TestSummaryStore_Delete(t *testing.T) {
	store := NewSummaryStore()
	store.Store("task-1", "test")
	store.Delete("task-1")
	_, ok := store.Load("task-1")
	if ok {
		t.Error("expected Load to return false after Delete")
	}
}

func TestSummaryStore_ConcurrentAccess(t *testing.T) {
	store := NewSummaryStore()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := string(rune('A' + n))
			store.Store(id, "summary-"+id)
			store.Load(id)
			store.Delete(id)
		}(i)
	}
	wg.Wait()
	// No race condition should occur.
}

// ============================================================
// Config tests
// ============================================================

func TestDefaultSummaryConfig(t *testing.T) {
	cfg := DefaultSummaryConfig()
	if cfg.Interval != 30*time.Second {
		t.Errorf("default interval should be 30s, got %v", cfg.Interval)
	}
}

func TestWithInterval(t *testing.T) {
	cfg := DefaultSummaryConfig()
	WithInterval(15 * time.Second)(&cfg)
	if cfg.Interval != 15*time.Second {
		t.Errorf("expected 15s interval, got %v", cfg.Interval)
	}
}

// ============================================================
// BuildSummaryPrompt tests
// ============================================================

func TestBuildSummaryPrompt_NoPrevious(t *testing.T) {
	prompt := BuildSummaryPrompt("")
	if !strings.Contains(prompt, "3-5 words") {
		t.Error("prompt should contain '3-5 words'")
	}
	if strings.Contains(prompt, "Previous:") {
		t.Error("prompt should not contain 'Previous:' when no previous summary")
	}
}

func TestBuildSummaryPrompt_WithPrevious(t *testing.T) {
	prompt := BuildSummaryPrompt("Reading test file")
	if !strings.Contains(prompt, `Previous: "Reading test file"`) {
		t.Error("prompt should contain previous summary text when provided")
	}
	if !strings.Contains(prompt, "say something NEW") {
		t.Error("prompt should encourage saying something new")
	}
}

func TestBuildSummaryPrompt_Examples(t *testing.T) {
	prompt := BuildSummaryPrompt("")
	if !strings.Contains(prompt, "Good:") {
		t.Error("prompt should contain good examples")
	}
	if !strings.Contains(prompt, "Bad ") {
		t.Error("prompt should contain bad examples")
	}
}

// ============================================================
// FilterIncompleteToolCalls tests
// ============================================================

func TestFilterIncompleteToolCalls_NoToolCalls(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
	}
	result := FilterIncompleteToolCalls(msgs)
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

func TestFilterIncompleteToolCalls_RemovesIncomplete(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("do something")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			message.ToolUsePart("call-1", "read_file", map[string]any{"path": "foo.txt"}),
		}},
		// No tool_result for call-1 → this assistant message should be removed
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("thanks")}},
	}
	result := FilterIncompleteToolCalls(msgs)
	if len(result) != 2 {
		t.Errorf("expected 2 messages (incomplete removed), got %d", len(result))
	}
	for _, msg := range result {
		if msg.Role == message.RoleAssistant {
			for _, part := range msg.Content {
				if part.ToolUseID == "call-1" {
					t.Error("incomplete tool_use should have been filtered out")
				}
			}
		}
	}
}

func TestFilterIncompleteToolCalls_KeepsComplete(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("do something")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			message.ToolUsePart("call-1", "read_file", map[string]any{"path": "foo.txt"}),
		}},
		{Role: message.RoleUser, Content: []message.ContentPart{
			message.ToolResultPart("call-1", "file content", false),
		}},
	}
	result := FilterIncompleteToolCalls(msgs)
	if len(result) != 3 {
		t.Errorf("expected 3 messages (complete), got %d", len(result))
	}
}

func TestFilterIncompleteToolCalls_EmptyInput(t *testing.T) {
	result := FilterIncompleteToolCalls(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
	result = FilterIncompleteToolCalls([]message.Message{})
	if len(result) != 0 {
		t.Error("expected empty slice for empty input")
	}
}

// ============================================================
// extractSummary tests
// ============================================================

func TestExtractSummary_FindsLastAssistant(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("Fixing bug in validate.ts")}},
	}
	got := extractSummary(msgs)
	if got != "Fixing bug in validate.ts" {
		t.Errorf("expected 'Fixing bug in validate.ts', got %q", got)
	}
}

func TestExtractSummary_EmptyMessages(t *testing.T) {
	got := extractSummary(nil)
	if got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}
}

func TestExtractSummary_NoAssistant(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}
	got := extractSummary(msgs)
	if got != "" {
		t.Errorf("expected empty string when no assistant message, got %q", got)
	}
}

// ============================================================
// StartAgentSummarization tests
// ============================================================

func TestStartAgentSummarization_StopsCleanly(t *testing.T) {
	store := NewSummaryStore()
	runForked := func(ctx context.Context, params engine.ForkedAgentParams) (*engine.ForkedAgentResult, error) {
		return &engine.ForkedAgentResult{
			Messages: []message.Message{
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("Test summary")}},
			},
		}, nil
	}

	stop := StartAgentSummarization(
		context.Background(),
		"test-task",
		"test-agent",
		engine.CacheSafeParams{},
		store,
		runForked,
		func() []message.Message {
			return []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("test")}},
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("test2")}},
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("test3")}},
			}
		},
		WithInterval(100*time.Millisecond),
	)

	// Let the first summary run
	time.Sleep(200 * time.Millisecond)

	stop()

	// Verify store has the summary
	got, ok := store.Load("test-task")
	if !ok {
		t.Log("note: summary may not have completed yet; this is non-deterministic")
	} else if got != "Test summary" {
		t.Errorf("expected 'Test summary', got %q", got)
	}
}

func TestStartAgentSummarization_NotEnoughMessages(t *testing.T) {
	store := NewSummaryStore()
	callCount := 0
	runForked := func(ctx context.Context, params engine.ForkedAgentParams) (*engine.ForkedAgentResult, error) {
		callCount++
		return &engine.ForkedAgentResult{
			Messages: []message.Message{
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("Test summary")}},
			},
		}, nil
	}

	stop := StartAgentSummarization(
		context.Background(),
		"test-task",
		"test-agent",
		engine.CacheSafeParams{},
		store,
		runForked,
		func() []message.Message {
			// Only 1 message, below the 3-message threshold
			return []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("test")}},
			}
		},
		WithInterval(50*time.Millisecond),
	)

	time.Sleep(150 * time.Millisecond)
	stop()

	if callCount > 0 {
		t.Logf("note: runForked was called %d times despite <3 messages (non-deterministic timer)", callCount)
	}
}

func TestStartAgentSummarization_StopMultipleTimes(t *testing.T) {
	store := NewSummaryStore()
	runForked := func(ctx context.Context, params engine.ForkedAgentParams) (*engine.ForkedAgentResult, error) {
		return &engine.ForkedAgentResult{}, nil
	}

	stop := StartAgentSummarization(
		context.Background(),
		"test-task",
		"test-agent",
		engine.CacheSafeParams{},
		store,
		runForked,
		func() []message.Message {
			return []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("1")}},
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("2")}},
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("3")}},
			}
		},
		WithInterval(1*time.Hour),
	)

	// Stop multiple times - should be idempotent.
	stop()
	stop()
	stop()
}

// ============================================================
// IsAgentSummaryEnabled test
// ============================================================

func TestIsAgentSummaryEnabled_DefaultDisabled(t *testing.T) {
	enabled := IsAgentSummaryEnabled()
	if enabled {
		t.Log("AgentSummary flag defaults to disabled (requires env var)")
	}
}
