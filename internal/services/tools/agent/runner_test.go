package agent

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
)

func TestFilterToolCatalog(t *testing.T) {
	catalog := []model.ToolDefinition{
		{Name: "Read"},
		{Name: "Write"},
		{Name: "Edit"},
		{Name: "Glob"},
		{Name: "Grep"},
	}

	tests := []struct {
		name       string
		disallowed []string
		wantNames  []string
	}{
		{
			name:       "no disallowed returns all",
			disallowed: nil,
			wantNames:  []string{"Read", "Write", "Edit", "Glob", "Grep"},
		},
		{
			name:       "empty disallowed returns all",
			disallowed: []string{},
			wantNames:  []string{"Read", "Write", "Edit", "Glob", "Grep"},
		},
		{
			name:       "filters single tool",
			disallowed: []string{"Write"},
			wantNames:  []string{"Read", "Edit", "Glob", "Grep"},
		},
		{
			name:       "filters multiple tools",
			disallowed: []string{"Write", "Edit", "Agent"},
			wantNames:  []string{"Read", "Glob", "Grep"},
		},
		{
			name:       "filters all tools",
			disallowed: []string{"Read", "Write", "Edit", "Glob", "Grep"},
			wantNames:  []string{},
		},
		{
			name:       "ignores unknown disallowed",
			disallowed: []string{"Unknown", "AlsoUnknown"},
			wantNames:  []string{"Read", "Write", "Edit", "Glob", "Grep"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterToolCatalog(catalog, tt.disallowed)
			if len(got) != len(tt.wantNames) {
				t.Fatalf("expected %d tools, got %d", len(tt.wantNames), len(got))
			}
			for i, want := range tt.wantNames {
				if got[i].Name != want {
					t.Errorf("tool[%d] = %q, want %q", i, got[i].Name, want)
				}
			}
			// Verify the original catalog is not mutated.
			if len(catalog) != 5 {
				t.Errorf("original catalog mutated: length %d, want 5", len(catalog))
			}
		})
	}
}

func TestSelectModel(t *testing.T) {
	parent := engine.New(nil, "parent-model", nil)
	runner := NewRunner(parent, nil)

	tests := []struct {
		name      string
		def       agent.Definition
		wantModel string
	}{
		{
			name:      "inherits parent model when agent model is empty",
			def:       agent.Definition{Model: ""},
			wantModel: "parent-model",
		},
		{
			name:      "inherits parent model when agent model is inherit",
			def:       agent.Definition{Model: "inherit"},
			wantModel: "parent-model",
		},
		{
			name:      "uses agent override model",
			def:       agent.Definition{Model: "haiku"},
			wantModel: "haiku",
		},
		{
			name:      "trims whitespace around agent model",
			def:       agent.Definition{Model: "  haiku  "},
			wantModel: "haiku",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.selectModel(tt.def)
			if got != tt.wantModel {
				t.Errorf("selectModel() = %q, want %q", got, tt.wantModel)
			}
		})
	}
}

func TestSelectModelNoParent(t *testing.T) {
	runner := NewRunner(nil, nil)
	got := runner.selectModel(agent.Definition{Model: ""})
	if got != "" {
		t.Errorf("selectModel() = %q, want empty string when parent is nil", got)
	}
}

func TestResolveMaxTurns(t *testing.T) {
	parent := engine.New(nil, "parent-model", nil)
	parent.MaxToolIterations = 12
	runner := NewRunner(parent, nil)

	tests := []struct {
		name     string
		def      agent.Definition
		wantTurns int
	}{
		{
			name:     "uses agent max turns",
			def:      agent.Definition{MaxTurns: 5},
			wantTurns: 5,
		},
		{
			name:     "falls back to parent max turns",
			def:      agent.Definition{MaxTurns: 0},
			wantTurns: 12,
		},
		{
			name:     "falls back to default when parent is nil",
			def:      agent.Definition{MaxTurns: 0},
			wantTurns: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.resolveMaxTurns(tt.def)
			if got != tt.wantTurns {
				t.Errorf("resolveMaxTurns() = %d, want %d", got, tt.wantTurns)
			}
		})
	}
}

func TestResolveMaxTurnsDefault(t *testing.T) {
	runner := NewRunner(nil, nil)
	got := runner.resolveMaxTurns(agent.Definition{MaxTurns: 0})
	if got != 8 {
		t.Errorf("resolveMaxTurns() = %d, want 8", got)
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	parent := engine.New(nil, "parent-model", nil)
	runner := NewRunner(parent, nil)

	// Definition without SystemPromptProvider returns empty string.
	got := runner.buildSystemPrompt(agent.Definition{}, coretool.UseContext{})
	if got != "" {
		t.Errorf("buildSystemPrompt() = %q, want empty string", got)
	}
}
