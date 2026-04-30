package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestLoremIpsumSkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerLoremIpsumSkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	meta := s.Metadata()
	if meta.Name != "lorem-ipsum" {
		t.Errorf("expected name 'lorem-ipsum', got %q", meta.Name)
	}
	if meta.Description == "" {
		t.Error("expected non-empty description")
	}

	// Test empty args (default 10000 tokens)
	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.Output == "" {
		t.Error("expected non-empty output for empty args")
	}

	// Test explicit token count
	result, err = s.Execute(context.Background(), command.Args{RawLine: "100"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.Output == "" {
		t.Error("expected non-empty output for 100 tokens")
	}

	// Test invalid input
	result, err = s.Execute(context.Background(), command.Args{RawLine: "-1"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Invalid token count") {
		t.Errorf("expected invalid token count message, got %q", result.Output)
	}

	// Test invalid input (non-numeric)
	result, err = s.Execute(context.Background(), command.Args{RawLine: "abc"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Invalid token count") {
		t.Errorf("expected invalid token count message, got %q", result.Output)
	}

	// Test cap at 500000
	result, err = s.Execute(context.Background(), command.Args{RawLine: "999999"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "capped at 500,000") {
		t.Errorf("expected cap message, got %q", result.Output)
	}
}
