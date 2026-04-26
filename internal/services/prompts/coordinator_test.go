package prompts

import (
	"context"
	"strings"
	"testing"
)

func TestCoordinatorSection_Compute_ReturnsEmptyWhenDisabled(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "")

	result, err := CoordinatorSection{}.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if result != "" {
		t.Fatalf("Compute() = %q, want empty string", result)
	}
}

func TestCoordinatorSection_Compute_RendersWorkerTools(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")

	ctx := WithRuntimeContext(context.Background(), RuntimeContext{
		EnabledToolNames: map[string]struct{}{
			"Read":            {},
			"Agent":           {},
			"Bash":            {},
			"SendMessage":     {},
			"TaskCreate":      {},
			"SyntheticOutput": {},
		},
	})

	result, err := CoordinatorSection{}.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	if !strings.Contains(result, "Workers spawned via the Agent tool have access to these tools: Bash, Read") {
		t.Fatalf("Compute() = %q, want worker tool summary", result)
	}
	if strings.Contains(result, "SendMessage") || strings.Contains(result, "SyntheticOutput") {
		t.Fatalf("Compute() = %q, want coordinator control tools excluded", result)
	}
}

func TestIdentitySection_SkipsInCoordinatorMode(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "true")

	result, err := IdentitySection{}.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if result != "" {
		t.Fatalf("Compute() = %q, want empty string in coordinator mode", result)
	}
}

func TestPromptBuilder_UsesCoordinatorSectionWhenEnabled(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")

	b := NewPromptBuilder(
		CoordinatorSection{},
		IdentitySection{},
	)

	result, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !strings.Contains(result, "You are Claude Code, an AI assistant that coordinates software engineering tasks across multiple workers.") {
		t.Fatalf("Build() = %q, want coordinator prompt", result)
	}
	if strings.Contains(result, "You are claude-code-go, an interactive agent") {
		t.Fatalf("Build() = %q, want default identity suppressed in coordinator mode", result)
	}
}
