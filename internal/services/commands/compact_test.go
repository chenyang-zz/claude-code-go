package commands

import (
	"context"
	"fmt"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestCompactCommandMetadata verifies /compact exposes stable metadata.
func TestCompactCommandMetadata(t *testing.T) {
	meta := CompactCommand{}.Metadata()
	if meta.Name != "compact" {
		t.Fatalf("Metadata().Name = %q, want compact", meta.Name)
	}
	if meta.Description != "Compact conversation history, keeping a summary in context" {
		t.Fatalf("Metadata().Description = %q, want stable compact description", meta.Description)
	}
	if meta.Usage != "/compact [instructions]" {
		t.Fatalf("Metadata().Usage = %q, want compact usage", meta.Usage)
	}
}

// TestCompactCommandExecuteFallback verifies /compact reports the fallback
// message when CompactFunc is nil.
func TestCompactCommandExecuteFallback(t *testing.T) {
	result, err := CompactCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Conversation compaction is not available yet. Use /clear to start a new session."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestCompactCommandExecuteWithFunc verifies /compact delegates to CompactFunc
// when set and passes custom instructions from args.
func TestCompactCommandExecuteWithFunc(t *testing.T) {
	var called bool
	var gotInstructions string
	cmd := CompactCommand{
		CompactFunc: func(ctx context.Context, instructions string) (string, error) {
			called = true
			gotInstructions = instructions
			return "Compacted successfully", nil
		},
	}

	result, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"focus on typescript changes"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("CompactFunc was not called")
	}
	if gotInstructions != "focus on typescript changes" {
		t.Fatalf("CompactFunc received instructions = %q, want %q", gotInstructions, "focus on typescript changes")
	}
	if result.Output != "Compacted successfully" {
		t.Fatalf("Execute() output = %q, want %q", result.Output, "Compacted successfully")
	}
}

// TestCompactCommandExecuteWithFuncError verifies /compact handles errors
// from CompactFunc gracefully.
func TestCompactCommandExecuteWithFuncError(t *testing.T) {
	cmd := CompactCommand{
		CompactFunc: func(ctx context.Context, instructions string) (string, error) {
			return "", fmt.Errorf("not enough messages to compact")
		},
	}

	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	want := "Error compacting conversation: not enough messages to compact"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
