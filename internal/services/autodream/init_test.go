package autodream

import (
	"context"
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestInitAutoDream_Disabled(t *testing.T) {
	// autoDream is disabled by default — Init should return nil.
	var registeredHook PostTurnHookFunc
	registerFn := func(hook PostTurnHookFunc) {
		registeredHook = hook
	}

	sys := InitAutoDream(nil, registerFn, "/tmp/test")
	if sys != nil {
		t.Error("expected nil system when autoDream is disabled")
	}
	if registeredHook != nil {
		t.Error("expected no hook registered when disabled")
	}
}

func TestInitAutoDream_Enabled(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_AUTO_DREAM", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_AUTO_DREAM")

	var registeredHook PostTurnHookFunc
	registerFn := func(hook PostTurnHookFunc) {
		registeredHook = hook
	}

	sys := InitAutoDream(nil, registerFn, "/tmp/test")
	if sys == nil {
		t.Fatal("expected non-nil system when autoDream is enabled")
	}
	if registeredHook == nil {
		t.Fatal("expected hook to be registered when enabled")
	}

	// Invoke the hook to verify it doesn't panic.
	err := registeredHook(context.Background(), []message.Message{}, "/tmp")
	if err != nil {
		t.Errorf("hook should not return error on first invocation: %v", err)
	}
}

func TestInitAutoDream_NilRunnerTolerated(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_AUTO_DREAM", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_AUTO_DREAM")

	var registeredHook PostTurnHookFunc
	registerFn := func(hook PostTurnHookFunc) {
		registeredHook = hook
	}

	sys := InitAutoDream(nil, registerFn, "/tmp/test")
	if sys == nil {
		t.Fatal("expected non-nil system even with nil runner")
	}

	// Invoke hook with nil runner — gate checks run, subagent execution skipped.
	err := registeredHook(context.Background(), []message.Message{}, "/tmp")
	if err != nil {
		t.Errorf("unexpected error with nil runner: %v", err)
	}
}
