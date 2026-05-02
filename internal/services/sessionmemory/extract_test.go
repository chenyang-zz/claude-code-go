package sessionmemory

import (
	"context"
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestIsSessionMemoryEnabled_Default(t *testing.T) {
	prev := os.Getenv("CLAUDE_FEATURE_SESSION_MEMORY")
	os.Unsetenv("CLAUDE_FEATURE_SESSION_MEMORY")
	defer os.Setenv("CLAUDE_FEATURE_SESSION_MEMORY", prev)

	if !IsSessionMemoryEnabled() {
		t.Error("expected IsSessionMemoryEnabled to return true by default")
	}
}

func TestIsSessionMemoryEnabled_Disabled(t *testing.T) {
	prev := os.Getenv("CLAUDE_FEATURE_SESSION_MEMORY")
	os.Setenv("CLAUDE_FEATURE_SESSION_MEMORY", "0")
	defer os.Setenv("CLAUDE_FEATURE_SESSION_MEMORY", prev)

	if IsSessionMemoryEnabled() {
		t.Error("expected IsSessionMemoryEnabled to return false for env '0'")
	}
}

func TestIsSessionMemoryEnabled_Enabled(t *testing.T) {
	prev := os.Getenv("CLAUDE_FEATURE_SESSION_MEMORY")
	os.Setenv("CLAUDE_FEATURE_SESSION_MEMORY", "1")
	defer os.Setenv("CLAUDE_FEATURE_SESSION_MEMORY", prev)

	if !IsSessionMemoryEnabled() {
		t.Error("expected IsSessionMemoryEnabled to return true for env '1'")
	}
}

func TestEstimateTokens(t *testing.T) {
	msgs := make([]message.Message, 5)
	result := estimateTokens(msgs)
	expected := 1000
	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestExtractSessionMemory_NonMainThread(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("extractSessionMemory panicked: %v", r)
		}
	}()

	ResetState()
	defer ResetState()

	err := extractSessionMemory(context.Background(), nil, nil, "")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// mockSubagentRunner implements SubagentRunner for testing.
type mockSubagentRunner struct{}

func (m *mockSubagentRunner) Run(_ context.Context, _ []message.Message) error {
	return nil
}

func TestSubagentRunner_Interface(t *testing.T) {
	var runner SubagentRunner = &mockSubagentRunner{}
	err := runner.Run(context.Background(), []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "hello"}}},
	})
	if err != nil {
		t.Errorf("mock runner returned error: %v", err)
	}
}
