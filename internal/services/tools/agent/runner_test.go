package agent

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
)

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
		name      string
		def       agent.Definition
		wantTurns int
	}{
		{
			name:      "uses agent max turns",
			def:       agent.Definition{MaxTurns: 5},
			wantTurns: 5,
		},
		{
			name:      "falls back to parent max turns",
			def:       agent.Definition{MaxTurns: 0},
			wantTurns: 12,
		},
		{
			name:      "falls back to default when parent is nil",
			def:       agent.Definition{MaxTurns: 0},
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

	// Definition without SystemPromptProvider returns only the tools note.
	got := runner.buildSystemPrompt(agent.Definition{}, coretool.UseContext{})
	want := "\n\nAvailable tools: All tools"
	if got != want {
		t.Errorf("buildSystemPrompt() = %q, want %q", got, want)
	}
}

func TestBuildSystemPromptUsesStaticPrompt(t *testing.T) {
	parent := engine.New(nil, "parent-model", nil)
	runner := NewRunner(parent, nil)

	got := runner.buildSystemPrompt(agent.Definition{
		SystemPrompt: "You are a custom agent.",
	}, coretool.UseContext{})
	want := "You are a custom agent.\n\nAvailable tools: All tools"
	if got != want {
		t.Errorf("buildSystemPrompt() = %q, want %q", got, want)
	}
}

func TestBuildAgentMessages(t *testing.T) {
	tests := []struct {
		name          string
		initialPrompt string
		prompt        string
		wantTexts     []string
	}{
		{
			name:          "prepends initial prompt before task prompt",
			initialPrompt: "Read the attached files first.",
			prompt:        "Summarize the implementation plan.",
			wantTexts:     []string{"Read the attached files first.", "Summarize the implementation plan."},
		},
		{
			name:          "skips blank initial prompt",
			initialPrompt: "   ",
			prompt:        "Summarize the implementation plan.",
			wantTexts:     []string{"Summarize the implementation plan."},
		},
		{
			name:          "keeps existing path when initial prompt is empty",
			initialPrompt: "",
			prompt:        "Summarize the implementation plan.",
			wantTexts:     []string{"Summarize the implementation plan."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAgentMessages(tt.initialPrompt, tt.prompt)
			if len(got) != len(tt.wantTexts) {
				t.Fatalf("buildAgentMessages() len = %d, want %d", len(got), len(tt.wantTexts))
			}
			for i, wantText := range tt.wantTexts {
				if got[i].Role != message.RoleUser {
					t.Fatalf("message[%d].Role = %q, want %q", i, got[i].Role, message.RoleUser)
				}
				if len(got[i].Content) != 1 {
					t.Fatalf("message[%d].Content len = %d, want 1", i, len(got[i].Content))
				}
				if got[i].Content[0].Text != wantText {
					t.Errorf("message[%d].Content[0].Text = %q, want %q", i, got[i].Content[0].Text, wantText)
				}
			}
		})
	}
}

// TestResolveAgentTools covers the full tool resolution logic including
// allowlist, denylist, wildcard, MCP passthrough, and built-in/custom
// default disallowed sets.
func TestConvertStopToSubagentStop(t *testing.T) {
	tests := []struct {
		name string
		cfg  hook.HooksConfig
		want hook.HooksConfig
	}{
		{
			name: "empty config",
			cfg:  nil,
			want: nil,
		},
		{
			name: "no Stop event",
			cfg: hook.HooksConfig{
				hook.EventSessionStart: {{Matcher: "test"}},
			},
			want: hook.HooksConfig{
				hook.EventSessionStart: {{Matcher: "test"}},
			},
		},
		{
			name: "Stop becomes SubagentStop",
			cfg: hook.HooksConfig{
				hook.EventStop: {{Matcher: "cleanup"}},
			},
			want: hook.HooksConfig{
				hook.EventSubagentStop: {{Matcher: "cleanup"}},
			},
		},
		{
			name: "mixed events",
			cfg: hook.HooksConfig{
				hook.EventStop:          {{Matcher: "cleanup"}},
				hook.EventSubagentStart: {{Matcher: "setup"}},
				hook.EventSessionStart:  {{Matcher: "init"}},
			},
			want: hook.HooksConfig{
				hook.EventSubagentStop:  {{Matcher: "cleanup"}},
				hook.EventSubagentStart: {{Matcher: "setup"}},
				hook.EventSessionStart:  {{Matcher: "init"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertStopToSubagentStop(tt.cfg)
			if len(got) != len(tt.want) {
				t.Fatalf("convertStopToSubagentStop() len = %d, want %d", len(got), len(tt.want))
			}
			for event, wantMatchers := range tt.want {
				gotMatchers, ok := got[event]
				if !ok {
					t.Errorf("missing event %q in result", event)
					continue
				}
				if len(gotMatchers) != len(wantMatchers) {
					t.Errorf("event %q: got %d matchers, want %d", event, len(gotMatchers), len(wantMatchers))
					continue
				}
				for i := range wantMatchers {
					if gotMatchers[i].Matcher != wantMatchers[i].Matcher {
						t.Errorf("event %q matcher[%d] = %q, want %q", event, i, gotMatchers[i].Matcher, wantMatchers[i].Matcher)
					}
				}
			}
		})
	}
}

func TestMergeAgentHooks_NoParent(t *testing.T) {
	runner := NewRunner(nil, nil)
	def := agent.Definition{
		Hooks: hook.HooksConfig{
			hook.EventStop: {{Matcher: "cleanup"}},
		},
	}
	got := runner.mergeAgentHooks(def)
	// When no parent, agent hooks are returned with Stop converted to SubagentStop
	want := hook.HooksConfig{
		hook.EventSubagentStop: {{Matcher: "cleanup"}},
	}
	if len(got) != len(want) {
		t.Fatalf("mergeAgentHooks() len = %d, want %d", len(got), len(want))
	}
	for event, wantMatchers := range want {
		gotMatchers, ok := got[event]
		if !ok {
			t.Fatalf("missing event %q", event)
		}
		if len(gotMatchers) != len(wantMatchers) {
			t.Fatalf("event %q: got %d matchers, want %d", event, len(gotMatchers), len(wantMatchers))
		}
	}
}

func TestMergeAgentHooks_NoAgentHooks(t *testing.T) {
	parent := engine.New(nil, "parent-model", nil)
	parent.Hooks = hook.HooksConfig{
		hook.EventSessionStart: {{Matcher: "init"}},
	}
	runner := NewRunner(parent, nil)
	def := agent.Definition{}
	got := runner.mergeAgentHooks(def)
	// When agent has no hooks, parent hooks are used as-is
	if len(got) != 1 {
		t.Fatalf("mergeAgentHooks() len = %d, want 1", len(got))
	}
	if _, ok := got[hook.EventSessionStart]; !ok {
		t.Errorf("expected EventSessionStart from parent, got events: %v", got)
	}
}

func TestMergeAgentHooks_Merge(t *testing.T) {
	parent := engine.New(nil, "parent-model", nil)
	parent.Hooks = hook.HooksConfig{
		hook.EventSessionStart: {{Matcher: "init"}},
	}
	runner := NewRunner(parent, nil)
	def := agent.Definition{
		Hooks: hook.HooksConfig{
			hook.EventSubagentStart: {{Matcher: "setup"}},
		},
	}
	got := runner.mergeAgentHooks(def)
	// Both parent and agent events should be present
	if len(got) != 2 {
		t.Fatalf("mergeAgentHooks() len = %d, want 2", len(got))
	}
	if _, ok := got[hook.EventSessionStart]; !ok {
		t.Errorf("missing EventSessionStart from parent")
	}
	if _, ok := got[hook.EventSubagentStart]; !ok {
		t.Errorf("missing EventSubagentStart from agent")
	}
}

func TestMergeAgentHooks_StopConversion(t *testing.T) {
	parent := engine.New(nil, "parent-model", nil)
	parent.Hooks = hook.HooksConfig{
		hook.EventSessionStart: {{Matcher: "init"}},
	}
	runner := NewRunner(parent, nil)
	def := agent.Definition{
		Hooks: hook.HooksConfig{
			hook.EventStop: {{Matcher: "cleanup"}},
		},
	}
	got := runner.mergeAgentHooks(def)
	// Stop should be converted to SubagentStop in the merged result
	if _, ok := got[hook.EventStop]; ok {
		t.Error("EventStop should not be present (converted to SubagentStop)")
	}
	if _, ok := got[hook.EventSubagentStop]; !ok {
		t.Error("EventSubagentStop should be present")
	}
	if _, ok := got[hook.EventSessionStart]; !ok {
		t.Error("EventSessionStart from parent should still be present")
	}
}

func TestResolveAgentTools(t *testing.T) {
	catalog := []model.ToolDefinition{
		{Name: "Read"},
		{Name: "Write"},
		{Name: "Edit"},
		{Name: "Glob"},
		{Name: "Grep"},
		{Name: "Agent"},
		{Name: "TaskStop"},
		{Name: "mcp__server1__tool1"},
		{Name: "mcp__server2__tool2"},
	}

	tests := []struct {
		name         string
		def          agent.Definition
		wantNames    []string
		wantWildcard bool
		wantInvalid  []string
	}{
		{
			name:         "built-in with no restrictions gets all except defaults",
			def:          agent.Definition{Source: "built-in"},
			wantNames:    []string{"Read", "Write", "Edit", "Glob", "Grep", "mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: true,
		},
		{
			name:         "custom with no restrictions gets all except defaults",
			def:          agent.Definition{Source: "userSettings"},
			wantNames:    []string{"Read", "Write", "Edit", "Glob", "Grep", "mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: true,
		},
		{
			name:         "wildcard explicit",
			def:          agent.Definition{Source: "built-in", Tools: []string{"*"}},
			wantNames:    []string{"Read", "Write", "Edit", "Glob", "Grep", "mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: true,
		},
		{
			name:         "allowlist filters to specific tools",
			def:          agent.Definition{Source: "built-in", Tools: []string{"Read", "Glob", "Grep"}},
			wantNames:    []string{"Read", "Glob", "Grep", "mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: false,
		},
		{
			name:         "allowlist with disallowed removes from intersection",
			def:          agent.Definition{Source: "built-in", Tools: []string{"Read", "Write", "Glob"}, DisallowedTools: []string{"Write"}},
			wantNames:    []string{"Read", "Glob", "mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: false,
		},
		{
			name:         "denylist only removes from full catalog",
			def:          agent.Definition{Source: "built-in", DisallowedTools: []string{"Write", "Edit"}},
			wantNames:    []string{"Read", "Glob", "Grep", "mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: true,
		},
		{
			name:         "MCP tools always pass through even with denylist",
			def:          agent.Definition{Source: "built-in", DisallowedTools: []string{"mcp__server1__tool1"}},
			wantNames:    []string{"Read", "Write", "Edit", "Glob", "Grep", "mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: true,
		},
		{
			name:         "MCP tools pass through allowlist",
			def:          agent.Definition{Source: "built-in", Tools: []string{"Read"}},
			wantNames:    []string{"Read", "mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: false,
		},
		{
			name:         "Agent tool blocked for built-in by default",
			def:          agent.Definition{Source: "built-in", Tools: []string{"Agent", "Read"}},
			wantNames:    []string{"Read", "mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: false,
		},
		{
			name:         "TaskStop blocked for custom by default",
			def:          agent.Definition{Source: "projectSettings", Tools: []string{"TaskStop", "Read"}},
			wantNames:    []string{"Read", "mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: false,
		},
		{
			name:         "invalid tools in allowlist tracked",
			def:          agent.Definition{Source: "built-in", Tools: []string{"Read", "NonExistent"}},
			wantNames:    []string{"Read", "mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: false,
			wantInvalid:  []string{"NonExistent"},
		},
		{
			name:         "empty allowlist returns MCP tools only",
			def:          agent.Definition{Source: "built-in", Tools: []string{}},
			wantNames:    []string{"mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: false,
		},
		{
			name:         "allowlist with only blocked tools returns MCP tools",
			def:          agent.Definition{Source: "built-in", Tools: []string{"Agent"}},
			wantNames:    []string{"mcp__server1__tool1", "mcp__server2__tool2"},
			wantWildcard: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAgentTools(tt.def, catalog)

			if got.HasWildcard != tt.wantWildcard {
				t.Errorf("HasWildcard = %v, want %v", got.HasWildcard, tt.wantWildcard)
			}

			if len(got.Tools) != len(tt.wantNames) {
				t.Fatalf("expected %d tools, got %d: %v", len(tt.wantNames), len(got.Tools), toolNames(got.Tools))
			}
			for i, want := range tt.wantNames {
				if got.Tools[i].Name != want {
					t.Errorf("tool[%d] = %q, want %q", i, got.Tools[i].Name, want)
				}
			}

			if len(tt.wantInvalid) > 0 {
				if len(got.InvalidSpecs) != len(tt.wantInvalid) {
					t.Errorf("invalid tools = %v, want %v", got.InvalidSpecs, tt.wantInvalid)
				}
				for i, want := range tt.wantInvalid {
					if got.InvalidSpecs[i] != want {
						t.Errorf("invalidTools[%d] = %q, want %q", i, got.InvalidSpecs[i], want)
					}
				}
			}
		})
	}
}
