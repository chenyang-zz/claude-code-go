package prompts

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
)

func TestAgentListingSection_Name(t *testing.T) {
	s := AgentListingSection{}
	if got := s.Name(); got != "agent_listing" {
		t.Errorf("Name() = %q, want %q", got, "agent_listing")
	}
}

func TestAgentListingSection_IsVolatile(t *testing.T) {
	s := AgentListingSection{}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
}

func TestAgentListingSection_Compute_EmptyRegistry(t *testing.T) {
	ctx := context.Background()

	// nil registry
	s := AgentListingSection{Registry: nil}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}
	if result != "" {
		t.Errorf("Compute() with nil registry = %q, want empty string", result)
	}

	// empty registry
	reg := agent.NewInMemoryRegistry()
	s = AgentListingSection{Registry: reg}
	result, err = s.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}
	if result != "" {
		t.Errorf("Compute() with empty registry = %q, want empty string", result)
	}
}

func TestAgentListingSection_Compute_SingleAgent(t *testing.T) {
	ctx := context.Background()
	reg := agent.NewInMemoryRegistry()

	_ = reg.Register(agent.Definition{
		AgentType: "test-runner",
		WhenToUse: "Use this agent to run tests after writing code",
	})

	s := AgentListingSection{Registry: reg}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}

	want := "Available agent types for the Agent tool:\n- test-runner: Use this agent to run tests after writing code (Tools: All tools)"
	if result != want {
		t.Errorf("Compute() = %q, want %q", result, want)
	}
}

func TestAgentListingSection_Compute_MultipleAgentsSorted(t *testing.T) {
	ctx := context.Background()
	reg := agent.NewInMemoryRegistry()

	_ = reg.Register(agent.Definition{
		AgentType: "zebra",
		WhenToUse: "Zebra agent",
	})
	_ = reg.Register(agent.Definition{
		AgentType: "alpha",
		WhenToUse: "Alpha agent",
	})
	_ = reg.Register(agent.Definition{
		AgentType: "beta",
		WhenToUse: "Beta agent",
	})

	s := AgentListingSection{Registry: reg}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}

	lines := strings.Split(result, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	// Header
	if lines[0] != "Available agent types for the Agent tool:" {
		t.Errorf("header = %q, want %q", lines[0], "Available agent types for the Agent tool:")
	}

	// Check alphabetical order: alpha, beta, zebra
	expected := []string{
		"- alpha: Alpha agent (Tools: All tools)",
		"- beta: Beta agent (Tools: All tools)",
		"- zebra: Zebra agent (Tools: All tools)",
	}
	for i, want := range expected {
		if lines[i+1] != want {
			t.Errorf("line %d = %q, want %q", i+1, lines[i+1], want)
		}
	}
}

func TestAgentListingSection_Compute_ToolsDescription_AllTools(t *testing.T) {
	ctx := context.Background()
	reg := agent.NewInMemoryRegistry()

	_ = reg.Register(agent.Definition{
		AgentType: "general-purpose",
		WhenToUse: "General-purpose agent for complex tasks",
	})

	s := AgentListingSection{Registry: reg}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}

	if !strings.Contains(result, "(Tools: All tools)") {
		t.Errorf("expected 'All tools' in result, got %q", result)
	}
}

func TestAgentListingSection_Compute_ToolsDescription_Denylist(t *testing.T) {
	ctx := context.Background()
	reg := agent.NewInMemoryRegistry()

	_ = reg.Register(agent.Definition{
		AgentType:       "explore",
		WhenToUse:       "Read-only search specialist",
		DisallowedTools: []string{"Agent", "Edit", "Write"},
	})

	s := AgentListingSection{Registry: reg}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}

	want := "(Tools: All tools except Agent, Edit, Write)"
	if !strings.Contains(result, want) {
		t.Errorf("expected %q in result, got %q", want, result)
	}
}

func TestAgentListingSection_Compute_ToolsDescription_Allowlist(t *testing.T) {
	ctx := context.Background()
	reg := agent.NewInMemoryRegistry()

	_ = reg.Register(agent.Definition{
		AgentType: "limited",
		WhenToUse: "Limited tool agent",
		Tools:     []string{"Read", "Bash"},
	})

	s := AgentListingSection{Registry: reg}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}

	want := "(Tools: Read, Bash)"
	if !strings.Contains(result, want) {
		t.Errorf("expected %q in result, got %q", want, result)
	}
}

func TestAgentListingSection_Compute_ToolsDescription_BothLists(t *testing.T) {
	ctx := context.Background()
	reg := agent.NewInMemoryRegistry()

	_ = reg.Register(agent.Definition{
		AgentType:       "filtered",
		WhenToUse:       "Filtered tool agent",
		Tools:           []string{"Read", "Bash", "Write"},
		DisallowedTools: []string{"Write"},
	})

	s := AgentListingSection{Registry: reg}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}

	want := "(Tools: Read, Bash)"
	if !strings.Contains(result, want) {
		t.Errorf("expected %q in result, got %q", want, result)
	}
}

func TestAgentListingSection_Compute_ToolsDescription_BothListsEmptyResult(t *testing.T) {
	ctx := context.Background()
	reg := agent.NewInMemoryRegistry()

	_ = reg.Register(agent.Definition{
		AgentType:       "blocked",
		WhenToUse:       "Blocked agent",
		Tools:           []string{"Read"},
		DisallowedTools: []string{"Read"},
	})

	s := AgentListingSection{Registry: reg}
	result, err := s.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}

	want := "(Tools: None)"
	if !strings.Contains(result, want) {
		t.Errorf("expected %q in result, got %q", want, result)
	}
}

func TestAgentListingSection_Integration_WithBuilder(t *testing.T) {
	ctx := context.Background()
	reg := agent.NewInMemoryRegistry()

	_ = reg.Register(agent.Definition{
		AgentType: "verify",
		WhenToUse: "Verification agent",
	})

	b := NewPromptBuilder(
		staticSection{name: "identity", content: "You are claude-code-go."},
		AgentListingSection{Registry: reg},
	)

	result, err := b.Build(ctx)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	expected := "You are claude-code-go.\n\nAvailable agent types for the Agent tool:\n- verify: Verification agent (Tools: All tools)"
	if result != expected {
		t.Errorf("Build() = %q, want %q", result, expected)
	}
}

func TestAgentListingSection_Integration_EmptySkipped(t *testing.T) {
	ctx := context.Background()
	reg := agent.NewInMemoryRegistry()

	b := NewPromptBuilder(
		staticSection{name: "identity", content: "You are claude-code-go."},
		AgentListingSection{Registry: reg},
	)

	result, err := b.Build(ctx)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	expected := "You are claude-code-go."
	if result != expected {
		t.Errorf("Build() = %q, want %q", result, expected)
	}
}
