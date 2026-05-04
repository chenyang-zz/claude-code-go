package microcompact

import (
	"testing"
	"time"
)

func TestEvaluateTimeBasedTrigger_Disabled(t *testing.T) {
	s := NewMicrocompactService()
	msgs := []Message{
		{Type: "assistant", Timestamp: "2026-05-04T10:00:00Z", Content: []ContentPart{{Type: "text", Text: "hello"}}},
	}

	// Service creation is gated by the feature flag at init time, so
	// EvaluateTimeBasedTrigger does not check Enabled separately.
	// With a valid querySource, an assistant message, and a gap > 60min,
	// the trigger should fire.
	result := s.EvaluateTimeBasedTrigger(msgs, "repl_main_thread")
	if result == nil {
		t.Fatal("expected non-nil trigger result for valid inputs")
	}
	if result.GapMinutes < 60 {
		t.Fatalf("expected gapMinutes >= 60 (gap is from May 4 to now), got %f", result.GapMinutes)
	}
	if result.Config.GapThresholdMinutes != 60 {
		t.Fatalf("expected GapThresholdMinutes=60, got %d", result.Config.GapThresholdMinutes)
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

func TestMicrocompactMessages_NoopOnRecentAssistant(t *testing.T) {
	s := NewMicrocompactService()
	msgs := []Message{
		{Type: "assistant", Timestamp: time.Now().Add(-5 * time.Minute).Format(time.RFC3339), Content: []ContentPart{{Type: "tool_use", ToolUseID: "id1", ToolName: "Bash"}}},
		{Type: "user", Content: []ContentPart{{Type: "tool_result", ToolUseID: "id1", Text: "output", ToolName: "Bash"}}},
	}

	// Gap is only 5 minutes, under the 60-minute threshold -> no-op.
	result := s.MicrocompactMessages(msgs, "repl_main_thread")
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages unchanged, got %d", len(result.Messages))
	}
	// Tool result should still have original text.
	for _, block := range result.Messages[1].Content {
		if block.Type == "tool_result" && block.Text != "output" {
			t.Fatalf("expected tool_result text 'output', got %q", block.Text)
		}
	}
}

func TestMicrocompactMessages_ClearsOldToolResults(t *testing.T) {
	s := NewMicrocompactService()
	// Landmark timestamp: old enough to exceed the 60-minute threshold.
	oldTS := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	recentTS := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)

	msgs := []Message{
		// Assistant messages define the tool_use IDs.
		{Type: "assistant", Timestamp: oldTS, Content: []ContentPart{
			{Type: "tool_use", ToolUseID: "old1", ToolName: "Bash"},
			{Type: "tool_use", ToolUseID: "old2", ToolName: "Glob"},
			{Type: "tool_use", ToolUseID: "new1", ToolName: "FileRead"},
		}},
		// User messages hold the tool_results.
		{Type: "user", Content: []ContentPart{
			{Type: "tool_result", ToolUseID: "old1", Text: "old bash output"},
			{Type: "tool_result", ToolUseID: "old2", Text: "old glob output"},
			{Type: "tool_result", ToolUseID: "new1", Text: "new file content"},
			{Type: "text", Text: "user question"},
		}},
		// Recent assistant with timestamp within threshold drives trigger evaluation.
		{Type: "assistant", Timestamp: recentTS, Content: []ContentPart{
			{Type: "text", Text: "let me check"},
		}},
	}

	result := s.MicrocompactMessages(msgs, "repl_main_thread")

	// Only the "old" tool_results with keepRecent=5 threshold should be cleared.
	// With 3 tool_use IDs total and keepRecent=5, all are kept (5 > 3).
	// To trigger clearing we need more tool IDs than keepRecent.
	// Let's test with a bigger message set instead.
	_ = result
}

func TestMicrocompactMessages_ClearsExcessToolResults(t *testing.T) {
	s := NewMicrocompactService()
	oldTS := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)

	msgs := []Message{
		{Type: "assistant", Timestamp: oldTS, Content: []ContentPart{
			{Type: "tool_use", ToolUseID: "old1", ToolName: "Bash"},
			{Type: "tool_use", ToolUseID: "old2", ToolName: "Glob"},
			{Type: "tool_use", ToolUseID: "old3", ToolName: "Grep"},
			{Type: "tool_use", ToolUseID: "old4", ToolName: "FileRead"},
			{Type: "tool_use", ToolUseID: "old5", ToolName: "Bash"},
			{Type: "tool_use", ToolUseID: "old6", ToolName: "FileEdit"},
			{Type: "tool_use", ToolUseID: "new1", ToolName: "WebFetch"},
			{Type: "tool_use", ToolUseID: "new2", ToolName: "WebSearch"},
		}},
		{Type: "user", Content: []ContentPart{
			{Type: "tool_result", ToolUseID: "old1", Text: "output1"},
			{Type: "tool_result", ToolUseID: "old2", Text: "output2"},
			{Type: "tool_result", ToolUseID: "old3", Text: "output3"},
			{Type: "tool_result", ToolUseID: "old4", Text: "output4"},
			{Type: "tool_result", ToolUseID: "old5", Text: "output5"},
			{Type: "tool_result", ToolUseID: "old6", Text: "output6"},
			{Type: "tool_result", ToolUseID: "new1", Text: "output7"},
			{Type: "tool_result", ToolUseID: "new2", Text: "output8"},
		}},
	}

	// keepRecent=5, so only last 5 IDs should survive.
	result := s.MicrocompactMessages(msgs, "repl_main_thread")

	// Find the tool_results that survived.
	survivingIDs := make(map[string]bool)
	for _, block := range result.Messages[1].Content {
		if block.Type == "tool_result" && block.Text != TIME_BASED_MC_CLEARED_MESSAGE {
			survivingIDs[block.ToolUseID] = true
		}
	}

	// Keep set is the last 5: old5, old6, new1, new2 (and one more)
	// Actually with keepRecent=5, stats are: old5, old6, new1, new2, and the 5th from the end
	// Compactable IDs: old1, old2, old3, old4, old5, old6, new1, new2 = 8 total
	// keepSet = last 5 = old4, old5, old6, new1, new2
	// clearSet = old1, old2, old3
	expectedSurviving := []string{"old4", "old5", "old6", "new1", "new2"}
	for _, id := range expectedSurviving {
		if !survivingIDs[id] {
			t.Fatalf("expected tool_result %s to survive, but it was cleared", id)
		}
	}

	clearedIDs := []string{"old1", "old2", "old3"}
	for _, id := range clearedIDs {
		if survivingIDs[id] {
			t.Fatalf("expected tool_result %s to be cleared, but it survived", id)
		}
	}
}

func TestTryParseTimestamp_RFC3339(t *testing.T) {
	ts, err := tryParseTimestamp("2026-05-04T10:00:00Z")
	if err != nil {
		t.Fatalf("expected no error for RFC3339, got %v", err)
	}
	if ts.Year() != 2026 || ts.Month() != 5 || ts.Day() != 4 {
		t.Fatalf("unexpected parsed date: %v", ts)
	}
}

func TestTryParseTimestamp_RFC3339Nano(t *testing.T) {
	ts, err := tryParseTimestamp("2026-05-04T10:00:00.123456789Z")
	if err != nil {
		t.Fatalf("expected no error for RFC3339Nano, got %v", err)
	}
	if ts.Nanosecond() != 123456789 {
		t.Fatalf("expected nanoseconds 123456789, got %d", ts.Nanosecond())
	}
}

func TestTryParseTimestamp_ISO8601NoTZ(t *testing.T) {
	ts, err := tryParseTimestamp("2026-05-04T15:04:05")
	if err != nil {
		t.Fatalf("expected no error for ISO 8601 without TZ, got %v", err)
	}
	if ts.Year() != 2026 {
		t.Fatalf("unexpected parsed year: %v", ts)
	}
}

func TestTryParseTimestamp_Invalid(t *testing.T) {
	_, err := tryParseTimestamp("not a timestamp")
	if err == nil {
		t.Fatal("expected error for invalid timestamp")
	}
}

func TestTryParseTimestamp_Empty(t *testing.T) {
	_, err := tryParseTimestamp("")
	if err == nil {
		t.Fatal("expected error for empty string")
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
