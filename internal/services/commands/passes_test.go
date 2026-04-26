package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestPassesCommandMetadata verifies /passes is exposed with the expected canonical descriptor.
func TestPassesCommandMetadata(t *testing.T) {
	meta := PassesCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "passes",
		Description: "Share a free week of Claude Code with friends",
		Usage:       "/passes",
	}) {
		t.Fatalf("Metadata() = %#v, want passes metadata", meta)
	}
}

// TestPassesCommandExecute verifies /passes returns the stable fallback.
func TestPassesCommandExecute(t *testing.T) {
	result, err := PassesCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != passesCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, passesCommandFallback)
	}
}
