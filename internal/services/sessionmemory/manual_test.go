package sessionmemory

import (
	"context"
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestManuallyExtractSessionMemory_EmptyMessages(t *testing.T) {
	ResetState()
	defer ResetState()

	// Test with nil messages.
	result := ManuallyExtractSessionMemory(context.Background(), nil)
	if result.Success {
		t.Errorf("expected Success to be false for nil messages, got true")
	}
	if result.Error != "No messages to summarize" {
		t.Errorf("expected error 'No messages to summarize', got: %s", result.Error)
	}

	// Test with empty messages slice.
	result = ManuallyExtractSessionMemory(context.Background(), []message.Message{})
	if result.Success {
		t.Errorf("expected Success to be false for empty messages, got true")
	}
	if result.Error != "No messages to summarize" {
		t.Errorf("expected error 'No messages to summarize', got: %s", result.Error)
	}
}

func TestManuallyExtractSessionMemory_Normal(t *testing.T) {
	ResetState()
	defer ResetState()

	// Use a temp directory to avoid polluting the real ~/.claude.
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "hello"}}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{{Type: "text", Text: "world"}}},
	}

	result := ManuallyExtractSessionMemory(context.Background(), msgs)
	if !result.Success {
		t.Errorf("expected Success to be true, got error: %s", result.Error)
	}
	if result.MemoryPath == "" {
		t.Error("expected MemoryPath to be non-empty")
	}
}

func TestMarkExtractionStartedCompleted(t *testing.T) {
	ResetState()
	defer ResetState()

	if IsExtractionInProgress() {
		t.Error("expected extraction not to be in progress initially")
	}

	MarkExtractionStarted()
	if !IsExtractionInProgress() {
		t.Error("expected extraction to be in progress after MarkExtractionStarted")
	}

	MarkExtractionCompleted()
	if IsExtractionInProgress() {
		t.Error("expected extraction not to be in progress after MarkExtractionCompleted")
	}
}
