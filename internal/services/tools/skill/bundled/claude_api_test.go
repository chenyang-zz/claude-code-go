package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestClaudeApiSkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerClaudeApiSkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "claude-api" {
		t.Errorf("expected name 'claude-api', got %q", s.Metadata().Name)
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Claude API") {
		t.Error("expected output to contain Claude API reference")
	}
	if !strings.Contains(result.Output, "WebFetch") {
		t.Error("expected output to reference WebFetch for latest docs")
	}

	result, err = s.Execute(context.Background(), command.Args{RawLine: "how do I use tool use?"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "how do I use tool use?") {
		t.Error("expected output to contain user request")
	}
}
