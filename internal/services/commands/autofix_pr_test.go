package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestAutofixPRCommandMetadata verifies /autofix-pr is exposed as a hidden command.
func TestAutofixPRCommandMetadata(t *testing.T) {
	meta := AutofixPRCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "autofix-pr",
		Description: "Run internal autofix PR workflow",
		Usage:       "/autofix-pr",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want autofix-pr metadata", meta)
	}
}

// TestAutofixPRCommandExecute verifies /autofix-pr returns the stable fallback for no-arg execution.
func TestAutofixPRCommandExecute(t *testing.T) {
	result, err := AutofixPRCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != autofixPRCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, autofixPRCommandFallback)
	}
}

// TestAutofixPRCommandExecuteRejectsArgs verifies /autofix-pr accepts no arguments.
func TestAutofixPRCommandExecuteRejectsArgs(t *testing.T) {
	_, err := AutofixPRCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /autofix-pr" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
