package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestColorCommandMetadata verifies /color is exposed with the expected canonical descriptor.
func TestColorCommandMetadata(t *testing.T) {
	meta := ColorCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "color",
		Description: "Set the prompt bar color for this session",
		Usage:       "/color <color|default>",
	}) {
		t.Fatalf("Metadata() = %#v, want color metadata", meta)
	}
}

// TestColorCommandExecute validates one argument and returns the stable /color fallback.
func TestColorCommandExecute(t *testing.T) {
	result, err := ColorCommand{}.Execute(context.Background(), command.Args{RawLine: "blue"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != colorCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, colorCommandFallback)
	}
}

// TestColorCommandExecuteRequiresColor verifies /color enforces one argument.
func TestColorCommandExecuteRequiresColor(t *testing.T) {
	_, err := ColorCommand{}.Execute(context.Background(), command.Args{})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /color <color|default>" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
