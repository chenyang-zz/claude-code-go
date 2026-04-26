package loader

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDefinitionFromFrontmatter_Minimal(t *testing.T) {
	fm := map[string]any{
		"name":        "custom-agent",
		"description": "A custom agent for testing",
	}
	def, err := BuildDefinitionFromFrontmatter("/project/.claude/agents/custom-agent.md", "/project/.claude/agents", fm, "System prompt here.", "projectSettings")
	require.NoError(t, err)
	assert.Equal(t, "custom-agent", def.AgentType)
	assert.Equal(t, "A custom agent for testing", def.WhenToUse)
	assert.Equal(t, "projectSettings", def.Source)
	assert.Equal(t, "/project/.claude/agents", def.BaseDir)
	assert.Equal(t, "custom-agent", def.Filename)
	assert.Equal(t, "System prompt here.", def.SystemPrompt)
}

func TestBuildDefinitionFromFrontmatter_MissingName(t *testing.T) {
	fm := map[string]any{
		"description": "Missing name",
	}
	_, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required 'name'")
}

func TestBuildDefinitionFromFrontmatter_MissingDescription(t *testing.T) {
	fm := map[string]any{
		"name": "agent",
	}
	_, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required 'description'")
}

func TestBuildDefinitionFromFrontmatter_NewlineEscape(t *testing.T) {
	fm := map[string]any{
		"name":        "agent",
		"description": "Line 1\\nLine 2",
	}
	def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
	require.NoError(t, err)
	assert.Equal(t, "Line 1\nLine 2", def.WhenToUse)
}

func TestBuildDefinitionFromFrontmatter_Tools(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		expected []string
	}{
		{"nil", nil, nil},
		{"empty_string", "", []string{}},
		{"single", "Read", []string{"Read"}},
		{"array", []any{"Read", "Grep"}, []string{"Read", "Grep"}},
		{"wildcard_string", "*", nil},
		{"wildcard_array", []any{"*"}, nil},
		{"mixed_with_wildcard", []any{"Read", "*"}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm := map[string]any{
				"name":        "agent",
				"description": "test",
				"tools":       tc.value,
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Tools)
		})
	}
}

func TestBuildDefinitionFromFrontmatter_Skills(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		expected []string
	}{
		{"nil", nil, nil},
		{"empty_string", "", nil},
		{"comma_separated", "skill1, skill2", []string{"skill1", "skill2"}},
		{"array", []any{"skill1", "skill2"}, []string{"skill1", "skill2"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm := map[string]any{
				"name":        "agent",
				"description": "test",
				"skills":      tc.value,
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Skills)
		})
	}
}

func TestBuildDefinitionFromFrontmatter_Model(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"sonnet", "sonnet"},
		{"  sonnet  ", "sonnet"},
		{"INHERIT", "inherit"},
		{"inherit", "inherit"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			fm := map[string]any{
				"name":        "agent",
				"description": "test",
				"model":       tc.input,
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Model)
		})
	}
}

func TestBuildDefinitionFromFrontmatter_Effort(t *testing.T) {
	cases := []struct {
		value    any
		expected string
	}{
		{"low", "low"},
		{"LOW", "low"},
		{3, "3"},
		{3.0, "3"},
		{"invalid", ""},
		{0, ""},
	}

	for _, tc := range cases {
		t.Run("effort", func(t *testing.T) {
			fm := map[string]any{
				"name":        "agent",
				"description": "test",
				"effort":      tc.value,
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Effort)
		})
	}
}

func TestBuildDefinitionFromFrontmatter_PermissionMode(t *testing.T) {
	cases := []struct {
		value    string
		expected string
	}{
		{"plan", "plan"},
		{"default", "default"},
		{"invalid", ""},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			fm := map[string]any{
				"name":           "agent",
				"description":    "test",
				"permissionMode": tc.value,
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.PermissionMode)
		})
	}
}

func TestBuildDefinitionFromFrontmatter_MaxTurns(t *testing.T) {
	cases := []struct {
		value    any
		expected int
	}{
		{10, 10},
		{10.0, 10},
		{"20", 20},
		{0, 0},
		{-1, 0},
		{"abc", 0},
	}

	for _, tc := range cases {
		t.Run("maxTurns", func(t *testing.T) {
			fm := map[string]any{
				"name":        "agent",
				"description": "test",
				"maxTurns":    tc.value,
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.MaxTurns)
		})
	}
}

func TestBuildDefinitionFromFrontmatter_Background(t *testing.T) {
	cases := []struct {
		value    any
		expected bool
	}{
		{true, true},
		{"true", true},
		{"True", true},
		{false, false},
		{"false", false},
		{"yes", false},
		{nil, false},
	}

	for _, tc := range cases {
		t.Run("background", func(t *testing.T) {
			fm := map[string]any{
				"name":        "agent",
				"description": "test",
				"background":  tc.value,
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Background)
		})
	}
}

func TestBuildDefinitionFromFrontmatter_Memory(t *testing.T) {
	cases := []struct {
		value    string
		expected string
	}{
		{"user", "user"},
		{"project", "project"},
		{"local", "local"},
		{"invalid", ""},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			fm := map[string]any{
				"name":        "agent",
				"description": "test",
				"memory":      tc.value,
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Memory)
		})
	}
}

func TestBuildDefinitionFromFrontmatter_Isolation(t *testing.T) {
	cases := []struct {
		value    string
		expected string
	}{
		{"worktree", "worktree"},
		{"remote", ""}, // remote is ant-only, excluded
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			fm := map[string]any{
				"name":        "agent",
				"description": "test",
				"isolation":   tc.value,
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Isolation)
		})
	}
}

func TestBuildDefinitionFromFrontmatter_InitialPrompt(t *testing.T) {
	fm := map[string]any{
		"name":          "agent",
		"description":   "test",
		"initialPrompt": "  Hello world  ",
	}
	def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
	require.NoError(t, err)
	assert.Equal(t, "Hello world", def.InitialPrompt)
}

func TestBuildDefinitionFromFrontmatter_Full(t *testing.T) {
	fm := map[string]any{
		"name":            "full-agent",
		"description":     "A full-featured agent",
		"tools":           []any{"Read", "Grep"},
		"disallowedTools": []any{"Edit"},
		"skills":          []any{"commit"},
		"model":           "sonnet",
		"effort":          "high",
		"permissionMode":  "plan",
		"maxTurns":        50,
		"background":      true,
		"memory":          "project",
		"isolation":       "worktree",
		"initialPrompt":   "Start here",
	}
	def, err := BuildDefinitionFromFrontmatter("/project/.claude/agents/full.md", "/project/.claude/agents", fm, "System prompt.", "projectSettings")
	require.NoError(t, err)
	assert.Equal(t, "full-agent", def.AgentType)
	assert.Equal(t, "A full-featured agent", def.WhenToUse)
	assert.Equal(t, []string{"Read", "Grep"}, def.Tools)
	assert.Equal(t, []string{"Edit"}, def.DisallowedTools)
	assert.Equal(t, []string{"commit"}, def.Skills)
	assert.Equal(t, "sonnet", def.Model)
	assert.Equal(t, "high", def.Effort)
	assert.Equal(t, "plan", def.PermissionMode)
	assert.Equal(t, 50, def.MaxTurns)
	assert.True(t, def.Background)
	assert.Equal(t, "project", def.Memory)
	assert.Equal(t, "worktree", def.Isolation)
	assert.Equal(t, "Start here", def.InitialPrompt)
	assert.Equal(t, "System prompt.", def.SystemPrompt)
}

func TestBuildDefinitionFromFrontmatter_Color(t *testing.T) {
	cases := []struct {
		value    string
		expected string
	}{
		{"red", "red"},
		{"BLUE", "blue"},
		{"  green  ", "green"},
		{"invalid", ""},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			fm := map[string]any{
				"name":        "agent",
				"description": "test",
				"color":       tc.value,
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Color)
		})
	}
}

func TestBuildDefinitionFromFrontmatter_MCPServers(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		expected []agent.AgentMCPServerSpec
	}{
		{
			name:     "nil",
			value:    nil,
			expected: nil,
		},
		{
			name:     "empty_array",
			value:    []any{},
			expected: nil,
		},
		{
			name:  "reference_only",
			value: []any{"slack", "github"},
			expected: []agent.AgentMCPServerSpec{
				{Name: "slack"},
				{Name: "github"},
			},
		},
		{
			name: "inline_definition",
			value: []any{
				map[string]any{
					"my-server": map[string]any{
						"type":    "stdio",
						"command": "npx",
						"args":    []any{"-y", "@modelcontextprotocol/server-filesystem"},
					},
				},
			},
			expected: []agent.AgentMCPServerSpec{
				{
					Name: "my-server",
					Config: map[string]any{
						"type":    "stdio",
						"command": "npx",
						"args":    []any{"-y", "@modelcontextprotocol/server-filesystem"},
					},
				},
			},
		},
		{
			name:  "mixed",
			value: []any{"slack", map[string]any{"inline": map[string]any{"type": "stdio", "command": "node"}}},
			expected: []agent.AgentMCPServerSpec{
				{Name: "slack"},
				{Name: "inline", Config: map[string]any{"type": "stdio", "command": "node"}},
			},
		},
		{
			name:     "invalid_non_array",
			value:    "not-an-array",
			expected: nil,
		},
		{
			name:     "invalid_elements_filtered",
			value:    []any{123, "valid", map[string]any{}},
			expected: []agent.AgentMCPServerSpec{{Name: "valid"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm := map[string]any{
				"name":        "agent",
				"description": "test",
			}
			if tc.value != nil {
				fm["mcpServers"] = tc.value
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.MCPServers)
		})
	}
}

func TestBuildDefinitionFromFrontmatter_Hooks(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		expected bool // whether Hooks should be non-nil
	}{
		{"nil", nil, false},
		{"empty_map", map[string]any{}, false},
		{
			"valid_hooks",
			map[string]any{
				"Stop": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "echo done"},
						},
					},
				},
			},
			true,
		},
		{
			"invalid_event_ignored",
			map[string]any{
				"UnknownEvent": []any{
					map[string]any{
						"hooks": []any{
							map[string]any{"type": "command", "command": "echo done"},
						},
					},
				},
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm := map[string]any{
				"name":        "agent",
				"description": "test",
			}
			if tc.value != nil {
				fm["hooks"] = tc.value
			}
			def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
			require.NoError(t, err)
			if tc.expected {
				assert.NotNil(t, def.Hooks)
			} else {
				assert.Nil(t, def.Hooks)
			}
		})
	}
}

func TestBuildDefinitionFromFrontmatter_AllNewFields(t *testing.T) {
	fm := map[string]any{
		"name":        "advanced-agent",
		"description": "An agent with all new fields",
		"color":       "purple",
		"mcpServers": []any{
			"slack",
			map[string]any{
				"custom-fs": map[string]any{
					"type":    "stdio",
					"command": "npx",
				},
			},
		},
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo stop"},
					},
				},
			},
		},
	}
	def, err := BuildDefinitionFromFrontmatter("test.md", "/agents", fm, "", "projectSettings")
	require.NoError(t, err)
	assert.Equal(t, "purple", def.Color)
	assert.Len(t, def.MCPServers, 2)
	assert.Equal(t, "slack", def.MCPServers[0].Name)
	assert.Equal(t, "custom-fs", def.MCPServers[1].Name)
	assert.NotNil(t, def.MCPServers[1].Config)
	assert.NotNil(t, def.Hooks)
	assert.True(t, def.Hooks.HasEvent("Stop"))
}

// Direct unit tests for new parser functions.

func TestParseAgentColor(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"red", "red"},
		{"BLUE", "blue"},
		{"  Green  ", "green"},
		{"yellow", "yellow"},
		{"purple", "purple"},
		{"orange", "orange"},
		{"pink", "pink"},
		{"cyan", "cyan"},
		{"invalid", ""},
		{"", ""},
		{"redd", ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := ParseAgentColor(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestParseAgentMCPServers(t *testing.T) {
	cases := []struct {
		name     string
		input    any
		expected []agent.AgentMCPServerSpec
	}{
		{"nil", nil, nil},
		{"not_array", "string", nil},
		{"empty_array", []any{}, nil},
		{
			"string_references",
			[]any{"slack", "github"},
			[]agent.AgentMCPServerSpec{{Name: "slack"}, {Name: "github"}},
		},
		{
			"inline_definition",
			[]any{map[string]any{"my-server": map[string]any{"type": "stdio", "command": "npx"}}},
			[]agent.AgentMCPServerSpec{{Name: "my-server", Config: map[string]any{"type": "stdio", "command": "npx"}}},
		},
		{
			"mixed",
			[]any{"slack", map[string]any{"inline": map[string]any{"type": "stdio"}}},
			[]agent.AgentMCPServerSpec{{Name: "slack"}, {Name: "inline", Config: map[string]any{"type": "stdio"}}},
		},
		{
			"invalid_elements_filtered",
			[]any{123, true, "valid", map[string]any{}},
			[]agent.AgentMCPServerSpec{{Name: "valid"}},
		},
		{
			"empty_string_skipped",
			[]any{"", "valid"},
			[]agent.AgentMCPServerSpec{{Name: "valid"}},
		},
		{
			"multi_key_map_skipped",
			[]any{map[string]any{"a": map[string]any{}, "b": map[string]any{}}},
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseAgentMCPServers(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestParseAgentHooks(t *testing.T) {
	cases := []struct {
		name     string
		input    any
		expected bool // whether result should be non-nil
	}{
		{"nil", nil, false},
		{"valid_command_hook", map[string]any{
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo hello"},
					},
				},
			},
		}, true},
		{"invalid_event_ignored", map[string]any{
			"UnknownEvent": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo hello"},
					},
				},
			},
		}, false},
		{"non_map", "not-a-map", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseAgentHooks(tc.input)
			if tc.expected {
				assert.NotNil(t, got)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}
