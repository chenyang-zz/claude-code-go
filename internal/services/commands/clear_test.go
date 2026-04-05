package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestClearCommandExecuteCreatesNewSession verifies /clear requests a fresh session switch with a stable confirmation line.
func TestClearCommandExecuteCreatesNewSession(t *testing.T) {
	result, err := ClearCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "Started a new session." {
		t.Fatalf("Execute() output = %q, want stable clear confirmation", result.Output)
	}
	if result.NewSessionID == "" {
		t.Fatal("Execute() new session id = empty, want generated session id")
	}
}
