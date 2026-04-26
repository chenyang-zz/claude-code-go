package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestThinkBackCommandMetadata verifies /think-back is exposed with the expected canonical descriptor.
func TestThinkBackCommandMetadata(t *testing.T) {
	meta := ThinkBackCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "think-back",
		Description: "Your 2025 Claude Code Year in Review",
		Usage:       "/think-back",
	}) {
		t.Fatalf("Metadata() = %#v, want think-back metadata", meta)
	}
}

// TestThinkBackCommandExecute verifies /think-back returns the stable fallback for no-arg execution.
func TestThinkBackCommandExecute(t *testing.T) {
	result, err := ThinkBackCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != thinkBackCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, thinkBackCommandFallback)
	}
}

// TestThinkBackCommandExecuteRejectsArgs verifies /think-back accepts no arguments.
func TestThinkBackCommandExecuteRejectsArgs(t *testing.T) {
	_, err := ThinkBackCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /think-back" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
