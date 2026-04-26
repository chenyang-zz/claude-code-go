package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestUltrareviewCommandMetadata verifies /ultrareview is exposed with the expected canonical descriptor.
func TestUltrareviewCommandMetadata(t *testing.T) {
	meta := UltrareviewCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "ultrareview",
		Description: "Find and verify bugs in your branch using Claude Code on the web",
		Usage:       "/ultrareview [pr-number]",
	}) {
		t.Fatalf("Metadata() = %#v, want ultrareview metadata", meta)
	}
}

// TestUltrareviewCommandExecute verifies /ultrareview returns the stable fallback for no-arg execution.
func TestUltrareviewCommandExecute(t *testing.T) {
	result, err := UltrareviewCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != ultrareviewCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, ultrareviewCommandFallback)
	}
}

// TestUltrareviewCommandExecuteAcceptsArgs verifies /ultrareview accepts optional PR number.
func TestUltrareviewCommandExecuteAcceptsArgs(t *testing.T) {
	result, err := UltrareviewCommand{}.Execute(context.Background(), command.Args{RawLine: "123"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != ultrareviewCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, ultrareviewCommandFallback)
	}
}
