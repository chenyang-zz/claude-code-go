package promptsuggestion

import (
	"context"
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestInit_Disabled(t *testing.T) {
	os.Setenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION", "0")
	defer os.Unsetenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION")

	var registeredHook PostSamplingHookFunc
	registerFn := func(hook PostSamplingHookFunc) {
		registeredHook = hook
	}

	suggester, cleanup := Init(nil, registerFn, "/tmp/test")
	if suggester != nil {
		t.Fatal("expected nil suggester when disabled")
	}
	if registeredHook != nil {
		t.Fatal("expected no hook registered when disabled")
	}

	// cleanup should not panic
	cleanup()
}

func TestInit_Enabled(t *testing.T) {
	os.Setenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION", "1")
	defer os.Unsetenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION")

	var registeredHook PostSamplingHookFunc
	registerFn := func(hook PostSamplingHookFunc) {
		registeredHook = hook
	}

	suggester, cleanup := Init(nil, registerFn, "/tmp/test")
	if suggester == nil {
		t.Fatal("expected non-nil suggester when enabled")
	}
	if registeredHook == nil {
		t.Fatal("expected hook to be registered")
	}

	// Invoke the registered hook to verify it doesn't panic
	ctx := context.Background()
	history := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}
	err := registeredHook(ctx, message.Message{Role: message.RoleAssistant}, history, "/tmp")
	if err != nil {
		t.Fatalf("expected hook to return nil error, got %v", err)
	}

	// cleanup should not panic
	cleanup()
}

func TestInit_CleanupUnregisters(t *testing.T) {
	os.Setenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION", "1")
	defer os.Unsetenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION")

	registerFn := func(hook PostSamplingHookFunc) {
		_ = hook
	}

	_, cleanup := Init(nil, registerFn, "/tmp/test")

	// cleanup should not panic
	cleanup()
}
