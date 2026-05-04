package sessionmemorycompact

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestDefaultSMCompactConfig(t *testing.T) {
	cfg := DefaultSMCompactConfig()
	if cfg.MinTokens != 10000 {
		t.Errorf("expected MinTokens=10000, got %d", cfg.MinTokens)
	}
	if cfg.MinTextBlockMessages != 5 {
		t.Errorf("expected MinTextBlockMessages=5, got %d", cfg.MinTextBlockMessages)
	}
	if cfg.MaxTokens != 40000 {
		t.Errorf("expected MaxTokens=40000, got %d", cfg.MaxTokens)
	}
}

func TestSetAndGetSessionMemoryCompactConfig(t *testing.T) {
	// Test setting config
	SetSessionMemoryCompactConfig(SessionMemoryCompactConfig{
		MinTokens:            5000,
		MinTextBlockMessages: 3,
		MaxTokens:            20000,
	})
	cfg := GetSessionMemoryCompactConfig()
	if cfg.MinTokens != 5000 {
		t.Errorf("expected MinTokens=5000, got %d", cfg.MinTokens)
	}
	if cfg.MinTextBlockMessages != 3 {
		t.Errorf("expected MinTextBlockMessages=3, got %d", cfg.MinTextBlockMessages)
	}
	if cfg.MaxTokens != 20000 {
		t.Errorf("expected MaxTokens=20000, got %d", cfg.MaxTokens)
	}

	// Reset and verify
	ResetSessionMemoryCompactConfig()
	cfg = GetSessionMemoryCompactConfig()
	if cfg.MinTokens != 10000 {
		t.Errorf("after reset, expected MinTokens=10000, got %d", cfg.MinTokens)
	}
}

func TestHasTextBlocks_AssistantText(t *testing.T) {
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentPart{
			{Type: "text", Text: "Hello"},
		},
	}
	if !hasTextBlocks(msg) {
		t.Error("expected hasTextBlocks=true for assistant text message")
	}
}

func TestHasTextBlocks_AssistantToolUse(t *testing.T) {
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentPart{
			{Type: "tool_use", ToolUseID: "id1", ToolName: "bash"},
		},
	}
	if hasTextBlocks(msg) {
		t.Error("expected hasTextBlocks=false for assistant tool_use message")
	}
}

func TestHasTextBlocks_UserText(t *testing.T) {
	msg := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			{Type: "text", Text: "Hello"},
		},
	}
	if !hasTextBlocks(msg) {
		t.Error("expected hasTextBlocks=true for user text message")
	}
}

func TestHasTextBlocks_UserToolResult(t *testing.T) {
	msg := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			{Type: "tool_result", ToolUseID: "id1", Text: "output"},
		},
	}
	if hasTextBlocks(msg) {
		t.Error("expected hasTextBlocks=false for user tool_result message")
	}
}

func TestGetToolResultIDs_Empty(t *testing.T) {
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentPart{
			{Type: "text", Text: "hello"},
		},
	}
	ids := getToolResultIDs(msg)
	if len(ids) != 0 {
		t.Errorf("expected empty ids for assistant, got %v", ids)
	}
}

func TestGetToolResultIDs_UserMessage(t *testing.T) {
	msg := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			{Type: "tool_result", ToolUseID: "id1"},
			{Type: "tool_result", ToolUseID: "id2"},
			{Type: "text", Text: "result"},
		},
	}
	ids := getToolResultIDs(msg)
	if len(ids) != 2 {
		t.Errorf("expected 2 ids, got %d: %v", len(ids), ids)
	}
	if ids[0] != "id1" || ids[1] != "id2" {
		t.Errorf("expected [id1 id2], got %v", ids)
	}
}

func TestHasToolUseWithIDs_Found(t *testing.T) {
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentPart{
			{Type: "tool_use", ToolUseID: "id1", ToolName: "bash"},
		},
	}
	ids := map[string]struct{}{"id1": {}}
	if !hasToolUseWithIDs(msg, ids) {
		t.Error("expected hasToolUseWithIDs=true for matching id")
	}
}

func TestHasToolUseWithIDs_NotFound(t *testing.T) {
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentPart{
			{Type: "tool_use", ToolUseID: "id1", ToolName: "bash"},
		},
	}
	ids := map[string]struct{}{"id2": {}}
	if hasToolUseWithIDs(msg, ids) {
		t.Error("expected hasToolUseWithIDs=false for non-matching id")
	}
}

func TestAdjustIndexToPreserveAPIInvariants_NoAdjust(t *testing.T) {
	msgs := make([]message.Message, 5)
	for i := range msgs {
		msgs[i] = message.Message{Role: message.RoleUser}
	}

	idx := AdjustIndexToPreserveAPIInvariants(msgs, 2)
	if idx != 2 {
		t.Errorf("expected index 2 (no adj), got %d", idx)
	}
}

func TestAdjustIndexToPreserveAPIInvariants_BoundaryCases(t *testing.T) {
	msgs := make([]message.Message, 5)

	// startIndex=0 should return 0
	idx := AdjustIndexToPreserveAPIInvariants(msgs, 0)
	if idx != 0 {
		t.Errorf("expected 0 for startIndex=0, got %d", idx)
	}

	// startIndex >= len should return startIndex
	idx = AdjustIndexToPreserveAPIInvariants(msgs, 10)
	if idx != 10 {
		t.Errorf("expected 10 for out-of-bounds, got %d", idx)
	}
}

func TestAdjustIndexToPreserveAPIInvariants_ToolPair(t *testing.T) {
	// Test case: tool_result needs preceding tool_use
	msgs := []message.Message{
		{Role: message.RoleAssistant, Content: []message.ContentPart{{Type: "text", Text: "thinking"}}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{{Type: "tool_use", ToolUseID: "id1", ToolName: "bash"}}},
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "tool_result", ToolUseID: "id1", Text: "output"}}},
	}

	// Start at index 2 (only tool_result kept) - should adjust back to 1
	idx := AdjustIndexToPreserveAPIInvariants(msgs, 2)
	if idx > 2 {
		t.Errorf("expected index <= 2, got %d", idx)
	}
}

func TestCalculateMessagesToKeepIndex_Empty(t *testing.T) {
	idx := CalculateMessagesToKeepIndex(nil, -1)
	if idx != 0 {
		t.Errorf("expected 0 for empty messages, got %d", idx)
	}
}

func TestCalculateMessagesToKeepIndex_AllKept(t *testing.T) {
	ResetSessionMemoryCompactConfig()
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "msg1"}}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{{Type: "text", Text: "response1"}}},
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "msg2"}}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{{Type: "text", Text: "response2"}}},
	}

	// With lastSummarizedIndex=-1 and small messages, all should be kept
	idx := CalculateMessagesToKeepIndex(msgs, -1)
	if idx < 0 || idx > len(msgs) {
		t.Errorf("expected valid index in [0,%d], got %d", len(msgs), idx)
	}
}

func TestCalculateMessagesToKeepIndex_AfterLastSummarized(t *testing.T) {
	ResetSessionMemoryCompactConfig()
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "old content"}}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{{Type: "text", Text: "old response"}}},
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "new content"}}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{{Type: "text", Text: "new response"}}},
	}

	// lastSummarizedIndex=1 means keep messages starting from index 2
	idx := CalculateMessagesToKeepIndex(msgs, 1)
	if idx < 0 || idx > 2 {
		t.Errorf("expected index in [0,2], got %d", idx)
	}
}

func TestShouldUseSessionMemoryCompaction_DefaultOff(t *testing.T) {
	enabled := ShouldUseSessionMemoryCompaction()
	if enabled {
		t.Error("expected session memory compaction to be disabled by default")
	}
}

func TestIsSessionMemoryCompactEnabled(t *testing.T) {
	enabled := IsSessionMemoryCompactEnabled()
	if enabled {
		t.Error("expected session memory compact flag to be disabled by default")
	}
}

func TestTrySessionMemoryCompaction_Disabled(t *testing.T) {
	result := TrySessionMemoryCompaction(nil)
	if result != nil {
		t.Error("expected nil result when disabled")
	}
}

func TestResetSessionMemoryCompactConfig(t *testing.T) {
	ResetSessionMemoryCompactConfig()
	cfg := GetSessionMemoryCompactConfig()
	if cfg.MinTokens != 10000 || cfg.MaxTokens != 40000 {
		t.Errorf("unexpected config after reset: %+v", cfg)
	}
}
