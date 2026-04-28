package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestBackfillSessionsCommandMetadata verifies /backfill-sessions is exposed as a hidden command.
func TestBackfillSessionsCommandMetadata(t *testing.T) {
	meta := BackfillSessionsCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "backfill-sessions",
		Description: "Backfill internal session fixtures",
		Usage:       "/backfill-sessions",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want backfill-sessions metadata", meta)
	}
}

// TestBackfillSessionsCommandExecute verifies /backfill-sessions returns the stable fallback for no-arg execution.
func TestBackfillSessionsCommandExecute(t *testing.T) {
	result, err := BackfillSessionsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != backfillSessionsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, backfillSessionsCommandFallback)
	}
}

// TestBackfillSessionsCommandExecuteRejectsArgs verifies /backfill-sessions accepts no arguments.
func TestBackfillSessionsCommandExecuteRejectsArgs(t *testing.T) {
	_, err := BackfillSessionsCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /backfill-sessions" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
