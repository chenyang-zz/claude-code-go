package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestPRCommentsCommandMetadata verifies /pr-comments exposes stable metadata.
func TestPRCommentsCommandMetadata(t *testing.T) {
	meta := PRCommentsCommand{}.Metadata()
	if meta.Name != "pr-comments" {
		t.Fatalf("Metadata().Name = %q, want pr-comments", meta.Name)
	}
	if meta.Description != "Get comments from a GitHub pull request" {
		t.Fatalf("Metadata().Description = %q, want stable pr-comments description", meta.Description)
	}
	if meta.Usage != "/pr-comments" {
		t.Fatalf("Metadata().Usage = %q, want /pr-comments", meta.Usage)
	}
}

// TestPRCommentsCommandExecuteReportsPluginNotice verifies /pr-comments returns the stable plugin migration notice.
func TestPRCommentsCommandExecuteReportsPluginNotice(t *testing.T) {
	result, err := PRCommentsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != prCommentsPluginNotice {
		t.Fatalf("Execute() output = %q, want %q", result.Output, prCommentsPluginNotice)
	}
}
