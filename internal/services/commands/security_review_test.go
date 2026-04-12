package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestSecurityReviewCommandMetadata verifies /security-review exposes stable metadata.
func TestSecurityReviewCommandMetadata(t *testing.T) {
	meta := SecurityReviewCommand{}.Metadata()
	if meta.Name != "security-review" {
		t.Fatalf("Metadata().Name = %q, want security-review", meta.Name)
	}
	if meta.Description != "Complete a security review of the pending changes on the current branch" {
		t.Fatalf("Metadata().Description = %q, want stable security-review description", meta.Description)
	}
	if meta.Usage != "/security-review" {
		t.Fatalf("Metadata().Usage = %q, want /security-review", meta.Usage)
	}
}

// TestSecurityReviewCommandExecuteReportsPluginNotice verifies /security-review returns the stable plugin migration notice.
func TestSecurityReviewCommandExecuteReportsPluginNotice(t *testing.T) {
	result, err := SecurityReviewCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != securityReviewPluginNotice {
		t.Fatalf("Execute() output = %q, want %q", result.Output, securityReviewPluginNotice)
	}
}
