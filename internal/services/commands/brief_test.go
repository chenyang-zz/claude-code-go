package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestBriefCommandMetadata verifies /brief is exposed with the expected canonical descriptor.
func TestBriefCommandMetadata(t *testing.T) {
	meta := BriefCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "brief",
		Description: "Toggle brief-only mode",
		Usage:       "/brief",
	}) {
		t.Fatalf("Metadata() = %#v, want brief metadata", meta)
	}
}

// TestBriefCommandExecute verifies /brief returns the stable fallback for no-arg execution.
func TestBriefCommandExecute(t *testing.T) {
	result, err := BriefCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != briefCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, briefCommandFallback)
	}
}

// TestBriefCommandExecuteRejectsArgs verifies /brief accepts no arguments.
func TestBriefCommandExecuteRejectsArgs(t *testing.T) {
	_, err := BriefCommand{}.Execute(context.Background(), command.Args{RawLine: "on"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /brief" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
