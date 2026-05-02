package extractmemories

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestIsModelVisibleMessage(t *testing.T) {
	if !isModelVisibleMessage(message.Message{Role: message.RoleUser}) {
		t.Error("user message should be visible")
	}
	if !isModelVisibleMessage(message.Message{Role: message.RoleAssistant}) {
		t.Error("assistant message should be visible")
	}
}

func TestCountModelVisibleMessagesSince(t *testing.T) {
	messages := []message.Message{
		{Role: message.RoleUser},
		{Role: message.RoleAssistant},
		{Role: message.RoleUser},
		{Role: message.RoleAssistant},
	}

	t.Run("count all when cursor is -1", func(t *testing.T) {
		n := countModelVisibleMessagesSince(messages, -1)
		if n != 4 {
			t.Errorf("expected 4, got %d", n)
		}
	})

	t.Run("count after index", func(t *testing.T) {
		n := countModelVisibleMessagesSince(messages, 1)
		if n != 2 {
			t.Errorf("expected 2, got %d", n)
		}
	})

	t.Run("cursor beyond length falls back to full count", func(t *testing.T) {
		n := countModelVisibleMessagesSince(messages, 100)
		if n != 4 {
			t.Errorf("expected 4 (fallback), got %d", n)
		}
	})
}

func TestHasMemoryWritesSince(t *testing.T) {
	projectRoot := "/Users/test/project"
	memDir := GetAutoMemPath(projectRoot)

	t.Run("no writes", func(t *testing.T) {
		messages := []message.Message{
			{Role: message.RoleUser},
			{Role: message.RoleAssistant},
		}
		if hasMemoryWritesSince(messages, -1, projectRoot) {
			t.Error("expected no memory writes")
		}
	})

	t.Run("write outside memory dir", func(t *testing.T) {
		messages := []message.Message{
			{Role: message.RoleAssistant, Content: []message.ContentPart{
				{Type: "tool_use", ToolName: "Write", ToolInput: map[string]any{"file_path": "/tmp/other.md"}},
			}},
		}
		if hasMemoryWritesSince(messages, -1, projectRoot) {
			t.Error("expected no memory writes for path outside memory dir")
		}
	})

	t.Run("write inside memory dir", func(t *testing.T) {
		insidePath := memDir + "user_role.md"
		messages := []message.Message{
			{Role: message.RoleAssistant, Content: []message.ContentPart{
				{Type: "tool_use", ToolName: "Write", ToolInput: map[string]any{"file_path": insidePath}},
			}},
		}
		if !hasMemoryWritesSince(messages, -1, projectRoot) {
			t.Error("expected memory write detection for path inside memory dir")
		}
	})

	t.Run("edit inside memory dir", func(t *testing.T) {
		insidePath := memDir + "feedback_testing.md"
		messages := []message.Message{
			{Role: message.RoleAssistant, Content: []message.ContentPart{
				{Type: "tool_use", ToolName: "Edit", ToolInput: map[string]any{"file_path": insidePath}},
			}},
		}
		if !hasMemoryWritesSince(messages, -1, projectRoot) {
			t.Error("expected memory write detection for Edit in memory dir")
		}
	})
}

func TestGetWrittenFilePath(t *testing.T) {
	t.Run("tool_use Write with file_path", func(t *testing.T) {
		block := message.ContentPart{
			Type:     "tool_use",
			ToolName: "Write",
			ToolInput: map[string]any{
				"file_path": "/tmp/test.md",
			},
		}
		fp := getWrittenFilePath(block)
		if fp != "/tmp/test.md" {
			t.Errorf("expected /tmp/test.md, got %q", fp)
		}
	})

	t.Run("text block returns empty", func(t *testing.T) {
		block := message.ContentPart{Type: "text", Text: "hello"}
		if getWrittenFilePath(block) != "" {
			t.Error("expected empty for text block")
		}
	})

	t.Run("Read tool_use returns empty", func(t *testing.T) {
		block := message.ContentPart{
			Type:     "tool_use",
			ToolName: "Read",
			ToolInput: map[string]any{
				"file_path": "/tmp/test.md",
			},
		}
		if getWrittenFilePath(block) != "" {
			t.Error("expected empty for Read tool_use")
		}
	})
}

type mockSubagentRunner struct {
	runCalled bool
	lastMsgs  []message.Message
}

func (m *mockSubagentRunner) Run(ctx context.Context, messages []message.Message) error {
	m.runCalled = true
	m.lastMsgs = messages
	return nil
}

func TestSystemExtractAfterTurn(t *testing.T) {
	// Use a temp project root to avoid real filesystem access.
	tmpDir := t.TempDir()
	runner := &mockSubagentRunner{}
	sys := NewSystem(runner, tmpDir)
	defer sys.ResetForTesting()

	messages := []message.Message{
		{Role: message.RoleUser},
		{Role: message.RoleAssistant},
	}

	// Should succeed (no-op since extractMemories feature gate is likely disabled).
	err := sys.extractAfterTurn(context.Background(), messages)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Wait for any background goroutine to complete.
	sys.DrainPendingExtraction(5000)
}

func TestSystemDrainPendingExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	sys := NewSystem(nil, tmpDir)
	defer sys.ResetForTesting()

	// Drain with no inflight should complete immediately.
	sys.DrainPendingExtraction(1000)
}

func TestExtractWrittenPaths(t *testing.T) {
	messages := []message.Message{
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "tool_use", ToolName: "Write", ToolInput: map[string]any{"file_path": "/mem/a.md"}},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "tool_use", ToolName: "Edit", ToolInput: map[string]any{"file_path": "/mem/b.md"}},
			{Type: "tool_use", ToolName: "Write", ToolInput: map[string]any{"file_path": "/mem/a.md"}}, // duplicate
		}},
	}

	paths := extractWrittenPaths(messages)
	if len(paths) != 2 {
		t.Errorf("expected 2 unique paths, got %d: %v", len(paths), paths)
	}
}
