package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestReviewCommandMetadata verifies /review is exposed with the expected canonical descriptor.
func TestReviewCommandMetadata(t *testing.T) {
	meta := ReviewCommand{}.Metadata()

	if meta.Name != "review" {
		t.Fatalf("Metadata().Name = %q, want review", meta.Name)
	}
	if meta.Description != "Review a pull request" {
		t.Fatalf("Metadata().Description = %q, want review description", meta.Description)
	}
	if meta.Usage != "/review [pr-number]" {
		t.Fatalf("Metadata().Usage = %q, want /review [pr-number]", meta.Usage)
	}
}

// TestReviewCommandExecute verifies /review returns the stable fallback.
func TestReviewCommandExecute(t *testing.T) {
	result, err := ReviewCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != reviewCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, reviewCommandFallback)
	}
}
