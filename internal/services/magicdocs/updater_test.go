package magicdocs

import (
	"context"
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// mockFileReader is a test double for FileReader that returns predefined content and error.
type mockFileReader struct {
	content string
	err     error
}

func (m *mockFileReader) ReadFile(filePath string) (string, error) {
	return m.content, m.err
}

// mockSubagentRunner is a test double for SubagentRunner that records invocations.
type mockSubagentRunner struct {
	called     bool
	lastPath   string
	lastPrompt string
	err        error
}

func (m *mockSubagentRunner) RunSubagent(ctx context.Context, filePath string, updatePrompt string, messages []message.Message) error {
	m.called = true
	m.lastPath = filePath
	m.lastPrompt = updatePrompt
	return m.err
}

// countingFileReader records whether ReadFile was called.
type countingFileReader struct {
	callCount int
	content   string
	err       error
}

func (m *countingFileReader) ReadFile(filePath string) (string, error) {
	m.callCount++
	return m.content, m.err
}

// TestUpdater_UpdateAllDocs_NormalFlow verifies that a registered Magic Doc
// triggers the subagent runner with the correct file path and a non-empty prompt.
func TestUpdater_UpdateAllDocs_NormalFlow(t *testing.T) {
	ClearTrackedMagicDocs()
	RegisterMagicDoc("/test/doc.md")

	runner := &mockSubagentRunner{}
	reader := &mockFileReader{
		content: "# MAGIC DOC: Test\n_Keep updated_\n\nSome content",
	}
	updater := NewUpdater(runner, reader)
	ctx := context.Background()
	msgs := []message.Message{}

	err := updater.UpdateAllDocs(ctx, msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !runner.called {
		t.Error("subagent runner was not called")
	}
	if runner.lastPath != "/test/doc.md" {
		t.Errorf("expected path /test/doc.md, got %s", runner.lastPath)
	}
	if runner.lastPrompt == "" {
		t.Error("expected non-empty update prompt")
	}
}

// TestUpdater_FileNotFound_Unregisters verifies that when a tracked Magic Doc
// file cannot be read (os.ErrNotExist), the subagent runner is not called and
// the file is removed from tracking.
func TestUpdater_FileNotFound_Unregisters(t *testing.T) {
	ClearTrackedMagicDocs()
	RegisterMagicDoc("/test/gone.md")

	runner := &mockSubagentRunner{}
	reader := &mockFileReader{
		err: os.ErrNotExist,
	}
	updater := NewUpdater(runner, reader)

	err := updater.UpdateAllDocs(context.Background(), []message.Message{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner.called {
		t.Error("runner should not be called when file does not exist")
	}
	if HasTrackedDocs() {
		t.Error("file should be unregistered when not found")
	}
}

// TestUpdater_HeaderRemoved_Unregisters verifies that when a tracked Magic Doc
// no longer contains the Magic Doc header, the subagent runner is not called
// and the file is removed from tracking.
func TestUpdater_HeaderRemoved_Unregisters(t *testing.T) {
	ClearTrackedMagicDocs()
	RegisterMagicDoc("/test/noheader.md")

	runner := &mockSubagentRunner{}
	reader := &mockFileReader{
		content: "# Just a normal heading\n\nSome markdown content without a magic doc header.",
	}
	updater := NewUpdater(runner, reader)

	err := updater.UpdateAllDocs(context.Background(), []message.Message{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner.called {
		t.Error("runner should not be called when header is missing")
	}
	if HasTrackedDocs() {
		t.Error("file should be unregistered when header is removed")
	}
}

// TestHasToolCallsInLastTurn verifies detection of pending tool calls in the
// most recent assistant message.
func TestHasToolCallsInLastTurn(t *testing.T) {
	t.Run("assistant with tool_use at end returns true", func(t *testing.T) {
		msgs := []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
			{Role: message.RoleAssistant, Content: []message.ContentPart{
				message.TextPart("sure"),
				message.ToolUsePart("tc1", "read", map[string]any{"path": "/f"}),
			}},
		}
		if !HasToolCallsInLastTurn(msgs) {
			t.Error("expected true when last assistant has tool_use")
		}
	})

	t.Run("assistant without tool_use returns false", func(t *testing.T) {
		msgs := []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
			{Role: message.RoleAssistant, Content: []message.ContentPart{
				message.TextPart("here is the answer"),
			}},
		}
		if HasToolCallsInLastTurn(msgs) {
			t.Error("expected false when last assistant has no tool_use")
		}
	})

	t.Run("empty messages returns false", func(t *testing.T) {
		if HasToolCallsInLastTurn([]message.Message{}) {
			t.Error("expected false for empty message list")
		}
	})

	t.Run("no assistant messages returns false", func(t *testing.T) {
		msgs := []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		}
		if HasToolCallsInLastTurn(msgs) {
			t.Error("expected false when there are no assistant messages")
		}
	})
}

// TestUpdater_NilRunner_NoPanic verifies that an Updater with a nil runner
// does not panic when UpdateAllDocs is called.
func TestUpdater_NilRunner_NoPanic(t *testing.T) {
	ClearTrackedMagicDocs()
	// No tracked docs, but even if there were, nil runner should return early.
	updater := NewUpdater(nil, &mockFileReader{content: "content"})
	ctx := context.Background()
	msgs := []message.Message{}

	// Should not panic.
	err := updater.UpdateAllDocs(ctx, msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestUpdater_NoTrackedDocs_Skips verifies that UpdateAllDocs returns early
// without calling the reader when no Magic Docs are tracked.
func TestUpdater_NoTrackedDocs_Skips(t *testing.T) {
	ClearTrackedMagicDocs()

	reader := &countingFileReader{
		content: "should not be read",
	}
	runner := &mockSubagentRunner{}
	updater := NewUpdater(runner, reader)

	err := updater.UpdateAllDocs(context.Background(), []message.Message{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.callCount > 0 {
		t.Errorf("reader should not be called when no docs are tracked, but was called %d times", reader.callCount)
	}
	if runner.called {
		t.Error("runner should not be called when no docs are tracked")
	}
}
