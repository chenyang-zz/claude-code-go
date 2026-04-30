package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestLoopSkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerLoopSkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "loop" {
		t.Errorf("expected name 'loop', got %q", s.Metadata().Name)
	}

	// Empty args returns usage
	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Usage: /loop") {
		t.Error("expected usage message for empty args")
	}

	// With args generates scheduling prompt
	result, err = s.Execute(context.Background(), command.Args{RawLine: "5m /babysit-prs"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "CronCreate") {
		t.Error("expected output to reference CronCreate")
	}

	// Interval-only (no prompt) returns usage
	result, err = s.Execute(context.Background(), command.Args{RawLine: "5m"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Usage: /loop") {
		t.Error("expected usage for interval-only input")
	}
}
