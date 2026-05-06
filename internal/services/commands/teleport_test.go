package commands

import (
	"context"
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

const teleportDisabledMessage = "Teleport command is not available in Claude Code Go yet. Remote handoff and teleport session flows remain unmigrated."

// TestTeleportCommandMetadata verifies /teleport is exposed as a hidden command.
func TestTeleportCommandMetadata(t *testing.T) {
	meta := TeleportCommand{}.Metadata()

	if meta.Name != "teleport" {
		t.Fatalf("Metadata().Name = %q, want \"teleport\"", meta.Name)
	}
	if !meta.Hidden {
		t.Fatal("Metadata().Hidden = false, want true")
	}
}

// TestTeleportCommandExecuteDisabled verifies /teleport returns the fallback
// when the feature flag is disabled.
func TestTeleportCommandExecuteDisabled(t *testing.T) {
	os.Unsetenv("CLAUDE_FEATURE_TELEPORT")

	result, err := TeleportCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != teleportDisabledMessage {
		t.Fatalf("Execute() output = %q, want %q", result.Output, teleportDisabledMessage)
	}
}

// TestTeleportCommandExecuteEnabledNoService verifies /teleport returns an error
// when the feature flag is enabled but service is not initialized.
func TestTeleportCommandExecuteEnabledNoService(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_TELEPORT", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_TELEPORT")

	_, err := TeleportCommand{}.Execute(context.Background(), command.Args{})
	if err == nil {
		t.Fatal("Execute() error = nil, want error when service is nil")
	}
}

// TestTeleportCommandRejectsArgs verifies /teleport with args is accepted
// (session ID mode) when flag is disabled, it still shows fallback.
func TestTeleportCommandRejectsArgs(t *testing.T) {
	os.Unsetenv("CLAUDE_FEATURE_TELEPORT")

	result, err := TeleportCommand{}.Execute(context.Background(), command.Args{RawLine: "session-1"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != teleportDisabledMessage {
		t.Fatalf("Execute() output = %q, want %q", result.Output, teleportDisabledMessage)
	}
}
