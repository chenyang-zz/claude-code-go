package loader

import (
	"testing"

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
