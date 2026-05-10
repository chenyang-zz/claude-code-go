package sessionmemory

import (
	"os"
	"testing"
)

func TestInitSessionMemory_Disabled(t *testing.T) {
	prev := os.Getenv("CLAUDE_FEATURE_SESSION_MEMORY")
	os.Setenv("CLAUDE_FEATURE_SESSION_MEMORY", "0")
	defer os.Setenv("CLAUDE_FEATURE_SESSION_MEMORY", prev)

	registerCount := 0
	InitSessionMemory(nil, func(hook PostTurnHookFunc) {
		registerCount++
	})

	if registerCount != 0 {
		t.Errorf("expected registerPostTurnHook not to be called, got %d calls", registerCount)
	}
}

func TestInitSessionMemory_Normal(t *testing.T) {
	prev := os.Getenv("CLAUDE_FEATURE_SESSION_MEMORY")
	os.Setenv("CLAUDE_FEATURE_SESSION_MEMORY", "1")
	defer os.Setenv("CLAUDE_FEATURE_SESSION_MEMORY", prev)

	var registeredHook PostTurnHookFunc
	registerCount := 0
	InitSessionMemory(nil, func(hook PostTurnHookFunc) {
		registerCount++
		registeredHook = hook
	})

	if registerCount != 1 {
		t.Errorf("expected registerPostTurnHook to be called once, got %d calls", registerCount)
	}
	if registeredHook == nil {
		t.Error("expected registered hook to be non-nil")
	}
}

func TestResetLastSummarizedMessageID(t *testing.T) {
	ResetState()
	defer ResetState()

	SetLastSummarizedMessageID("test-id-123")
	if GetLastSummarizedMessageID() != "test-id-123" {
		t.Errorf("expected 'test-id-123', got: %s", GetLastSummarizedMessageID())
	}

	ResetLastSummarizedMessageID()
	if GetLastSummarizedMessageID() != "" {
		t.Errorf("expected empty string after reset, got: %s", GetLastSummarizedMessageID())
	}
}
