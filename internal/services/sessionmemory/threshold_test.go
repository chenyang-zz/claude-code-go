package sessionmemory

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// stubTool is a minimal test double implementing the tool.Tool interface.
type stubTool struct {
	name string
}

func (s stubTool) Name() string                      { return s.name }
func (s stubTool) Description() string               { return "" }
func (s stubTool) InputSchema() tool.InputSchema     { return tool.InputSchema{} }
func (s stubTool) IsReadOnly() bool                   { return true }
func (s stubTool) IsConcurrencySafe() bool            { return true }
func (s stubTool) Invoke(_ context.Context, _ tool.Call) (tool.Result, error) {
	return tool.Result{}, nil
}

// makeMsg constructs a message.Message for testing. When hasToolCalls is true,
// a tool_use content block is appended to the message content.
func makeMsg(role message.Role, hasToolCalls bool) message.Message {
	content := []message.ContentPart{message.TextPart("content")}
	if hasToolCalls {
		content = append(content, message.ContentPart{Type: "tool_use"})
	}
	return message.Message{Role: role, Content: content}
}

// TestHasMetInitializationThreshold_Met verifies that a token count equal to
// MinimumMessageTokensToInit (10000) returns true.
func TestHasMetInitializationThreshold_Met(t *testing.T) {
	ResetState()
	cfg := GetSessionMemoryConfig()

	result := HasMetInitializationThreshold(cfg.MinimumMessageTokensToInit)

	if !result {
		t.Errorf("HasMetInitializationThreshold(%d) = false, want true", cfg.MinimumMessageTokensToInit)
	}
}

// TestHasMetInitializationThreshold_NotMet verifies that a token count below
// MinimumMessageTokensToInit returns false.
func TestHasMetInitializationThreshold_NotMet(t *testing.T) {
	ResetState()

	result := HasMetInitializationThreshold(5000)

	if result {
		t.Errorf("HasMetInitializationThreshold(5000) = true, want false")
	}
}

// TestHasMetUpdateThreshold_Met verifies that 10000 tokens against a baseline
// of 0 exceeds MinimumTokensBetweenUpdate (5000), returning true.
func TestHasMetUpdateThreshold_Met(t *testing.T) {
	ResetState()

	result := HasMetUpdateThreshold(10000)

	if !result {
		t.Error("HasMetUpdateThreshold(10000) = false, want true (growth >= 5000)")
	}
}

// TestHasMetUpdateThreshold_NotMet verifies that 1000 tokens against a
// baseline of 0 does not meet the update threshold.
func TestHasMetUpdateThreshold_NotMet(t *testing.T) {
	ResetState()

	result := HasMetUpdateThreshold(1000)

	if result {
		t.Error("HasMetUpdateThreshold(1000) = true, want false (growth < 5000)")
	}
}

// TestCountToolCallsSince verifies tool_use content blocks are counted
// correctly across assistant messages.
func TestCountToolCallsSince(t *testing.T) {
	msgs := []message.Message{
		makeMsg(message.RoleUser, false),
		makeMsg(message.RoleAssistant, true), // 1 tool_use
		makeMsg(message.RoleAssistant, true), // 1 tool_use
	}

	count := CountToolCallsSince(msgs, "")

	if count != 2 {
		t.Errorf("CountToolCallsSince() = %d, want 2", count)
	}
}

// TestCountToolCallsSince_Empty verifies that messages without any tool_use
// content blocks return a count of 0.
func TestCountToolCallsSince_Empty(t *testing.T) {
	msgs := []message.Message{
		makeMsg(message.RoleUser, false),
		makeMsg(message.RoleAssistant, false),
	}

	count := CountToolCallsSince(msgs, "")

	if count != 0 {
		t.Errorf("CountToolCallsSince() = %d, want 0", count)
	}
}

// TestHasToolCallsInLastAssistantTurn verifies that when the most recent
// assistant message contains tool_use blocks, the function returns true.
func TestHasToolCallsInLastAssistantTurn(t *testing.T) {
	msgs := []message.Message{
		makeMsg(message.RoleUser, false),
		makeMsg(message.RoleAssistant, true), // last assistant has tool_use
	}

	result := HasToolCallsInLastAssistantTurn(msgs)

	if !result {
		t.Error("HasToolCallsInLastAssistantTurn() = false, want true (last assistant has tool_use)")
	}
}

// TestHasToolCallsInLastAssistantTurn_NoCalls verifies that when the most
// recent assistant message lacks tool_use blocks, the function returns false.
func TestHasToolCallsInLastAssistantTurn_NoCalls(t *testing.T) {
	msgs := []message.Message{
		makeMsg(message.RoleUser, false),
		makeMsg(message.RoleAssistant, false), // last assistant, no tool_use
		makeMsg(message.RoleUser, false),
	}

	result := HasToolCallsInLastAssistantTurn(msgs)

	if result {
		t.Error("HasToolCallsInLastAssistantTurn() = true, want false (last assistant has no tool_use)")
	}
}

// TestShouldExtractMemory_EmptyMessages verifies that an empty message slice
// returns false because there are no messages to evaluate.
func TestShouldExtractMemory_EmptyMessages(t *testing.T) {
	ResetState()

	result := ShouldExtractMemory([]message.Message{}, func(msgs []message.Message) int {
		return 0
	})

	if result {
		t.Error("ShouldExtractMemory() with empty messages = true, want false")
	}
}

// TestCreateMemoryFileCanUseTool verifies the access control function
// returned by CreateMemoryFileCanUseTool:
//   - FileEditTool ("Edit") with the correct memory path is allowed
//   - FileEditTool with a different file path is denied
//   - A non-FileEdit tool is denied
func TestCreateMemoryFileCanUseTool(t *testing.T) {
	memoryPath := "/path/to/memory-notes.md"
	canUse := CreateMemoryFileCanUseTool(memoryPath)

	// FileEditTool with correct path should be allowed.
	editTool := stubTool{name: "Edit"}
	allowed, reason := canUse(editTool, map[string]any{"file_path": memoryPath})
	if !allowed {
		t.Errorf("FileEditTool with correct path should be allowed, got reason: %s", reason)
	}

	// FileEditTool with a different path should be denied.
	allowed, reason = canUse(editTool, map[string]any{"file_path": "/wrong/path.md"})
	if allowed {
		t.Error("FileEditTool with wrong path should be denied")
	}
	if !strings.Contains(reason, memoryPath) {
		t.Errorf("Denial reason should mention the correct path %q, got: %s", memoryPath, reason)
	}

	// A non-FileEdit tool should be denied.
	otherTool := stubTool{name: "Bash"}
	allowed, reason = canUse(otherTool, map[string]any{"file_path": memoryPath})
	if allowed {
		t.Error("Non-FileEdit tool should be denied")
	}
	if !strings.Contains(reason, "Edit") {
		t.Errorf("Denial reason should mention only Edit tool is allowed, got: %s", reason)
	}
}
