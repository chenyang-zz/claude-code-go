package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestBatchSkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerBatchSkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "batch" {
		t.Errorf("expected name 'batch', got %q", s.Metadata().Name)
	}

	// Empty args returns missing instruction message
	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Provide an instruction") {
		t.Error("expected missing instruction message for empty args")
	}

	// With args generates batch prompt
	result, err = s.Execute(context.Background(), command.Args{RawLine: "migrate from react to vue"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "migrate from react to vue") {
		t.Error("expected output to contain instruction")
	}
	if !strings.Contains(result.Output, "Batch: Parallel Work Orchestration") {
		t.Error("expected output to contain batch header")
	}
}
