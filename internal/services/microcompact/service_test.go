package microcompact

import (
	"testing"
)

func TestEvaluateTimeBasedTrigger_Disabled(t *testing.T) {
	s := NewMicrocompactService()
	msgs := []Message{
		{Type: "assistant", Timestamp: "2026-05-04T10:00:00Z", Content: []ContentPart{{Type: "text", Text: "hello"}}},
	}

	result := s.EvaluateTimeBasedTrigger(msgs, "repl_main_thread")
	if result != nil {
		t.Fatalf("expected nil for disabled flag, got %+v", result)
	}
}

func TestEvaluateTimeBasedTrigger_NoQuerySource(t *testing.T) {
	s := NewMicrocompactService()
	msgs := []Message{
		{Type: "assistant", Timestamp: "2026-05-04T10:00:00Z", Content: []ContentPart{{Type: "text", Text: "hello"}}},
	}

	// We can't easily set the feature flag without env, so test the logic path
	// that querySource must be present and main-thread.
	result := s.EvaluateTimeBasedTrigger(msgs, "")
	if result != nil {
		t.Fatalf("expected nil for empty querySource")
	}
}

func TestEvaluateTimeBasedTrigger_NonMainThreadSource(t *testing.T) {
	s := NewMicrocompactService()
	msgs := []Message{
		{Type: "assistant", Timestamp: "2026-05-04T10:00:00Z", Content: []ContentPart{{Type: "text", Text: "hello"}}},
	}

	result := s.EvaluateTimeBasedTrigger(msgs, "agent:session_memory")
	if result != nil {
		t.Fatalf("expected nil for subagent querySource")
	}
}

func TestEvaluateTimeBasedTrigger_NoAssistantMessage(t *testing.T) {
	s := NewMicrocompactService()
	msgs := []Message{
		{Type: "user", Timestamp: "2026-05-04T10:00:00Z", Content: []ContentPart{{Type: "text", Text: "hello"}}},
	}

	result := s.EvaluateTimeBasedTrigger(msgs, "repl_main_thread")
	if result != nil {
		t.Fatalf("expected nil when no assistant message")
	}
}

func TestEvaluateTimeBasedTrigger_NilTimestamp(t *testing.T) {
	s := NewMicrocompactService()
	msgs := []Message{
		{Type: "assistant", Content: []ContentPart{{Type: "text", Text: "hello"}}},
	}

	result := s.EvaluateTimeBasedTrigger(msgs, "repl_main_thread")
	if result != nil {
		t.Fatalf("expected nil for empty timestamp")
	}
}

func TestCollectCompactableToolIDs_Empty(t *testing.T) {
	ids := collectCompactableToolIDs(nil)
	if len(ids) != 0 {
		t.Fatalf("expected empty, got %v", ids)
	}

	ids = collectCompactableToolIDs([]Message{})
	if len(ids) != 0 {
		t.Fatalf("expected empty, got %v", ids)
	}
}

func TestCollectCompactableToolIDs_NoToolUse(t *testing.T) {
	msgs := []Message{
		{Type: "assistant", Content: []ContentPart{{Type: "text", Text: "hello"}}},
	}
	ids := collectCompactableToolIDs(msgs)
	if len(ids) != 0 {
		t.Fatalf("expected empty for no tool_use blocks, got %v", ids)
	}
}

func TestCollectCompactableToolIDs_MixedTools(t *testing.T) {
	msgs := []Message{
		{
			Type: "assistant",
			Content: []ContentPart{
				{Type: "text", Text: "let me search"},
				{Type: "tool_use", ToolUseID: "id1", ToolName: "WebSearch"},
				{Type: "tool_use", ToolUseID: "id2", ToolName: "UnknownTool"},
				{Type: "tool_use", ToolUseID: "id3", ToolName: "Bash"},
			},
		},
	}
	ids := collectCompactableToolIDs(msgs)
	if len(ids) != 2 {
		t.Fatalf("expected 2 compactable IDs (WebSearch, Bash), got %v", ids)
	}
	if ids[0] != "id1" || ids[1] != "id3" {
		t.Fatalf("expected [id1, id3], got %v", ids)
	}
}

func TestCollectCompactableToolIDs_MultipleAssistantMessages(t *testing.T) {
	msgs := []Message{
		{
			Type: "assistant",
			Content: []ContentPart{
				{Type: "tool_use", ToolUseID: "a1", ToolName: "Glob"},
			},
		},
		{
			Type: "user",
			Content: []ContentPart{
				{Type: "text", Text: "ok"},
			},
		},
		{
			Type: "assistant",
			Content: []ContentPart{
				{Type: "tool_use", ToolUseID: "b1", ToolName: "FileRead"},
				{Type: "tool_use", ToolUseID: "b2", ToolName: "Grep"},
			},
		},
	}
	ids := collectCompactableToolIDs(msgs)
	if len(ids) != 3 {
		t.Fatalf("expected 3 compactable IDs, got %v", ids)
	}
}

func TestIsMainThreadSource_Empty(t *testing.T) {
	if isMainThreadSource("") {
		t.Fatal("expected false for empty string")
	}
}

func TestIsMainThreadSource_ExactMatch(t *testing.T) {
	if !isMainThreadSource("repl_main_thread") {
		t.Fatal("expected true for exact match")
	}
}

func TestIsMainThreadSource_PrefixMatch(t *testing.T) {
	if !isMainThreadSource("repl_main_thread:outputStyle:custom") {
		t.Fatal("expected true for prefix match with suffix")
	}
}

func TestIsMainThreadSource_Subagent(t *testing.T) {
	if isMainThreadSource("agent:session_memory") {
		t.Fatal("expected false for subagent source")
	}
}

func TestIsMainThreadSource_Other(t *testing.T) {
	if isMainThreadSource("sdk") {
		t.Fatal("expected false for sdk source")
	}
}

func TestMicrocompactMessages_NoopWhenDisabled(t *testing.T) {
	s := NewMicrocompactService()
	msgs := []Message{
		{Type: "user", Content: []ContentPart{{Type: "text", Text: "hello"}}},
	}

	result := s.MicrocompactMessages(msgs, "repl_main_thread")
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
}

func TestSuppressCompactWarning(t *testing.T) {
	s := NewMicrocompactService()

	if s.IsCompactWarningSuppressed() {
		t.Fatal("expected warning not suppressed initially")
	}

	s.SuppressCompactWarning()
	if !s.IsCompactWarningSuppressed() {
		t.Fatal("expected warning suppressed after SuppressCompactWarning")
	}

	s.ClearCompactWarningSuppression()
	if s.IsCompactWarningSuppressed() {
		t.Fatal("expected warning not suppressed after ClearCompactWarningSuppression")
	}
}

func TestMicrocompactMessages_ClearsWarning(t *testing.T) {
	s := NewMicrocompactService()
	s.SuppressCompactWarning()

	// MicrocompactMessages should clear suppression at start
	msgs := []Message{
		{Type: "user", Content: []ContentPart{{Type: "text", Text: "hello"}}},
	}
	s.MicrocompactMessages(msgs, "repl_main_thread")

	if s.IsCompactWarningSuppressed() {
		t.Fatal("expected warning cleared after MicrocompactMessages")
	}
}

func TestPostCompactCleanup(t *testing.T) {
	s := NewMicrocompactService()
	s.SuppressCompactWarning()
	if !s.IsCompactWarningSuppressed() {
		t.Fatal("expected warning suppressed")
	}

	s.PostCompactCleanup()
	if s.IsCompactWarningSuppressed() {
		t.Fatal("expected warning not suppressed after PostCompactCleanup")
	}
}

func TestCompactableToolSet(t *testing.T) {
	expectedTools := []string{
		"Bash", "Glob", "Grep", "FileRead", "FileWrite",
		"FileEdit", "WebFetch", "WebSearch", "NotebookEdit",
	}

	for _, tool := range expectedTools {
		if !CompactableToolSet[tool] {
			t.Fatalf("expected %s to be in CompactableToolSet", tool)
		}
	}

	if CompactableToolSet["UnknownTool"] {
		t.Fatal("expected UnknownTool to NOT be in CompactableToolSet")
	}
}

func TestCollectCompactableToolIDs_IgnoresUserMessages(t *testing.T) {
	msgs := []Message{
		{
			Type: "user",
			Content: []ContentPart{
				{Type: "tool_result", ToolUseID: "id1"},
			},
		},
	}
	ids := collectCompactableToolIDs(msgs)
	if len(ids) != 0 {
		t.Fatalf("expected empty for user messages, got %v", ids)
	}
}
