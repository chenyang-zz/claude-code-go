package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestGoodClaudeCommandMetadata verifies /good-claude is exposed as a hidden command.
func TestGoodClaudeCommandMetadata(t *testing.T) {
	meta := GoodClaudeCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "good-claude",
		Description: "Run internal good-claude workflow",
		Usage:       "/good-claude",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want good-claude metadata", meta)
	}
}

// TestGoodClaudeCommandExecute verifies /good-claude returns the stable fallback for no-arg execution.
func TestGoodClaudeCommandExecute(t *testing.T) {
	result, err := GoodClaudeCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != goodClaudeCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, goodClaudeCommandFallback)
	}
}

// TestGoodClaudeCommandExecuteRejectsArgs verifies /good-claude accepts no arguments.
func TestGoodClaudeCommandExecuteRejectsArgs(t *testing.T) {
	_, err := GoodClaudeCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /good-claude" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
