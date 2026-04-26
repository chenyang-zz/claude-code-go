package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestSkillsCommandMetadata verifies /skills is exposed with the expected canonical descriptor.
func TestSkillsCommandMetadata(t *testing.T) {
	meta := SkillsCommand{}.Metadata()

	if meta.Name != "skills" {
		t.Fatalf("Metadata().Name = %q, want skills", meta.Name)
	}
	if meta.Description != "List available skills" {
		t.Fatalf("Metadata().Description = %q, want skills description", meta.Description)
	}
	if meta.Usage != "/skills" {
		t.Fatalf("Metadata().Usage = %q, want /skills", meta.Usage)
	}
}

// TestSkillsCommandExecute verifies /skills returns the stable fallback.
func TestSkillsCommandExecute(t *testing.T) {
	result, err := SkillsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != skillsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, skillsCommandFallback)
	}
}
