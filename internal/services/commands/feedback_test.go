package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestFeedbackCommandMetadata verifies /feedback is exposed with the expected canonical descriptor.
func TestFeedbackCommandMetadata(t *testing.T) {
	meta := FeedbackCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "feedback",
		Aliases:     []string{"bug"},
		Description: "Submit feedback about Claude Code",
		Usage:       "/feedback [report]",
	}) {
		t.Fatalf("Metadata() = %#v, want feedback metadata", meta)
	}
}

// TestFeedbackCommandExecute verifies /feedback returns the stable fallback.
func TestFeedbackCommandExecute(t *testing.T) {
	result, err := FeedbackCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != feedbackCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, feedbackCommandFallback)
	}
}
