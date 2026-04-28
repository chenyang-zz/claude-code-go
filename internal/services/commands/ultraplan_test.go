package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestUltraplanCommandMetadata verifies /ultraplan is exposed as a hidden command.
func TestUltraplanCommandMetadata(t *testing.T) {
	meta := UltraplanCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "ultraplan",
		Description: "Draft an advanced plan in Claude Code on the web",
		Usage:       "/ultraplan <prompt>",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want ultraplan metadata", meta)
	}
}

// TestUltraplanCommandExecute verifies /ultraplan returns the stable fallback when a prompt is provided.
func TestUltraplanCommandExecute(t *testing.T) {
	result, err := UltraplanCommand{}.Execute(context.Background(), command.Args{RawLine: "refine release checklist"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != ultraplanCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, ultraplanCommandFallback)
	}
}

// TestUltraplanCommandExecuteRejectsEmptyPrompt verifies /ultraplan requires a prompt argument.
func TestUltraplanCommandExecuteRejectsEmptyPrompt(t *testing.T) {
	_, err := UltraplanCommand{}.Execute(context.Background(), command.Args{})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /ultraplan <prompt>" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
