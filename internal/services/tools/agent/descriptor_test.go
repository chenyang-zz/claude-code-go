package agent

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
)

func TestDescriptor_Description_NilRegistry(t *testing.T) {
	d := &Descriptor{Registry: nil}
	got := d.Description()
	want := "Launch a specialized agent to perform a task. Use this when you need to delegate work to a subagent."
	if got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
}

func TestDescriptor_Description_EmptyRegistry(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	d := &Descriptor{Registry: registry}
	got := d.Description()
	want := "Launch a specialized agent to perform a task. Use this when you need to delegate work to a subagent."
	if got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
}

func TestDescriptor_Description_SingleAgent(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	_ = registry.Register(agent.Definition{
		AgentType: "explore",
		WhenToUse: "Read-only search specialist",
		Tools:     []string{"Read", "Bash", "Grep", "Glob"},
	})

	d := &Descriptor{Registry: registry}
	got := d.Description()

	// Verify it contains key sections
	if !strings.Contains(got, "Launch a specialized agent to handle complex, multi-step tasks autonomously.") {
		t.Error("missing opening paragraph")
	}
	if !strings.Contains(got, "- explore: Read-only search specialist (Tools: Read, Bash, Grep, Glob)") {
		t.Error("missing or incorrect agent listing")
	}
	if !strings.Contains(got, "When NOT to use the Agent tool:") {
		t.Error("missing 'When NOT to use' section")
	}
	if !strings.Contains(got, "Usage notes:") {
		t.Error("missing 'Usage notes' section")
	}
	if !strings.Contains(got, "Example usage:") {
		t.Error("missing 'Example usage' section")
	}
}

func TestDescriptor_Description_MultipleAgents_Sorted(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	_ = registry.Register(agent.Definition{
		AgentType: "verify",
		WhenToUse: "Verification agent",
	})
	_ = registry.Register(agent.Definition{
		AgentType: "explore",
		WhenToUse: "Read-only search specialist",
	})
	_ = registry.Register(agent.Definition{
		AgentType: "test-runner",
		WhenToUse: "Run tests after code changes",
	})

	d := &Descriptor{Registry: registry}
	got := d.Description()

	// Verify alphabetical order: explore, test-runner, verify
	exploreIdx := strings.Index(got, "- explore:")
	testRunnerIdx := strings.Index(got, "- test-runner:")
	verifyIdx := strings.Index(got, "- verify:")

	if exploreIdx == -1 || testRunnerIdx == -1 || verifyIdx == -1 {
		t.Fatal("missing one or more agent entries")
	}
	if !(exploreIdx < testRunnerIdx && testRunnerIdx < verifyIdx) {
		t.Error("agents are not sorted alphabetically")
	}
}

func TestDescriptor_Description_ToolsAllowlistOnly(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	_ = registry.Register(agent.Definition{
		AgentType: "limited",
		WhenToUse: "Limited tool agent",
		Tools:     []string{"Read", "Bash"},
	})

	d := &Descriptor{Registry: registry}
	got := d.Description()

	if !strings.Contains(got, "(Tools: Read, Bash)") {
		t.Errorf("expected allowlist tools, got: %s", got)
	}
}

func TestDescriptor_Description_ToolsDenylistOnly(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	_ = registry.Register(agent.Definition{
		AgentType:       "restricted",
		WhenToUse:       "Restricted agent",
		DisallowedTools: []string{"Edit", "Write"},
	})

	d := &Descriptor{Registry: registry}
	got := d.Description()

	if !strings.Contains(got, "(Tools: All tools except Edit, Write)") {
		t.Errorf("expected denylist tools description, got: %s", got)
	}
}

func TestDescriptor_Description_ToolsBothLists(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	_ = registry.Register(agent.Definition{
		AgentType:       "filtered",
		WhenToUse:       "Filtered agent",
		Tools:           []string{"Read", "Bash", "Edit"},
		DisallowedTools: []string{"Edit", "Write"},
	})

	d := &Descriptor{Registry: registry}
	got := d.Description()

	if !strings.Contains(got, "(Tools: Read, Bash)") {
		t.Errorf("expected filtered tools (Edit removed), got: %s", got)
	}
}

func TestDescriptor_Description_ToolsBothListsEmptyResult(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	_ = registry.Register(agent.Definition{
		AgentType:       "none",
		WhenToUse:       "No tools agent",
		Tools:           []string{"Edit"},
		DisallowedTools: []string{"Edit"},
	})

	d := &Descriptor{Registry: registry}
	got := d.Description()

	if !strings.Contains(got, "(Tools: None)") {
		t.Errorf("expected 'None' for empty effective tools, got: %s", got)
	}
}

func TestDescriptor_Description_ToolsNoLists(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	_ = registry.Register(agent.Definition{
		AgentType: "alltools",
		WhenToUse: "All tools agent",
	})

	d := &Descriptor{Registry: registry}
	got := d.Description()

	if !strings.Contains(got, "(Tools: All tools)") {
		t.Errorf("expected 'All tools' when no lists, got: %s", got)
	}
}

func TestFormatAgentLine(t *testing.T) {
	def := agent.Definition{
		AgentType: "test",
		WhenToUse: "Test agent",
		Tools:     []string{"Read", "Bash"},
	}
	got := formatAgentLine(def)
	want := "- test: Test agent (Tools: Read, Bash)"
	if got != want {
		t.Errorf("formatAgentLine() = %q, want %q", got, want)
	}
}

func TestGetToolsDescription(t *testing.T) {
	tests := []struct {
		name            string
		tools           []string
		disallowedTools []string
		want            string
	}{
		{
			name:            "neither list",
			want:            "All tools",
		},
		{
			name:  "allowlist only",
			tools: []string{"Read", "Bash"},
			want:  "Read, Bash",
		},
		{
			name:            "denylist only",
			disallowedTools: []string{"Edit", "Write"},
			want:            "All tools except Edit, Write",
		},
		{
			name:            "both lists",
			tools:           []string{"Read", "Bash", "Edit"},
			disallowedTools: []string{"Edit"},
			want:            "Read, Bash",
		},
		{
			name:            "both lists empty result",
			tools:           []string{"Edit"},
			disallowedTools: []string{"Edit"},
			want:            "None",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getToolsDescription(tt.tools, tt.disallowedTools)
			if got != tt.want {
				t.Errorf("getToolsDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}
