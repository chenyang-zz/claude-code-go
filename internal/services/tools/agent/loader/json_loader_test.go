package loader

import (
	"encoding/json"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAgentFromJson_Minimal(t *testing.T) {
	jsonData := []byte(`{
		"description": "A test agent",
		"prompt": "You are a test agent."
	}`)

	def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
	require.NoError(t, err)
	assert.Equal(t, "test-agent", def.AgentType)
	assert.Equal(t, "A test agent", def.WhenToUse)
	assert.Equal(t, "You are a test agent.", def.SystemPrompt)
	assert.Equal(t, "flagSettings", def.Source)
}

func TestParseAgentFromJson_DefaultSource(t *testing.T) {
	jsonData := []byte(`{
		"description": "A test agent",
		"prompt": "You are a test agent."
	}`)

	def, err := ParseAgentFromJson("test-agent", jsonData, "")
	require.NoError(t, err)
	assert.Equal(t, "flagSettings", def.Source)
}

func TestParseAgentFromJson_MissingDescription(t *testing.T) {
	jsonData := []byte(`{
		"prompt": "You are a test agent."
	}`)

	_, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required 'description'")
}

func TestParseAgentFromJson_MissingPrompt(t *testing.T) {
	jsonData := []byte(`{
		"description": "A test agent"
	}`)

	_, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required 'prompt'")
}

func TestParseAgentFromJson_EmptyDescription(t *testing.T) {
	jsonData := []byte(`{
		"description": "   ",
		"prompt": "You are a test agent."
	}`)

	_, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required 'description'")
}

func TestParseAgentFromJson_EmptyPrompt(t *testing.T) {
	jsonData := []byte(`{
		"description": "A test agent",
		"prompt": "   "
	}`)

	_, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required 'prompt'")
}

func TestParseAgentFromJson_Full(t *testing.T) {
	jsonData := []byte(`{
		"description": "A full-featured agent",
		"prompt": "You are a full agent.",
		"tools": ["Read", "Grep"],
		"disallowedTools": ["Edit"],
		"skills": ["commit", "test"],
		"model": "sonnet",
		"effort": "high",
		"permissionMode": "plan",
		"maxTurns": 50,
		"background": true,
		"memory": "project",
		"isolation": "worktree",
		"initialPrompt": "Start here",
		"color": "purple"
	}`)

	def, err := ParseAgentFromJson("full-agent", jsonData, "userSettings")
	require.NoError(t, err)
	assert.Equal(t, "full-agent", def.AgentType)
	assert.Equal(t, "A full-featured agent", def.WhenToUse)
	assert.Equal(t, "You are a full agent.", def.SystemPrompt)
	assert.Equal(t, "userSettings", def.Source)
	assert.Equal(t, []string{"Read", "Grep"}, def.Tools)
	assert.Equal(t, []string{"Edit"}, def.DisallowedTools)
	assert.Equal(t, []string{"commit", "test"}, def.Skills)
	assert.Equal(t, "sonnet", def.Model)
	assert.Equal(t, "high", def.Effort)
	assert.Equal(t, "plan", def.PermissionMode)
	assert.Equal(t, 50, def.MaxTurns)
	assert.True(t, def.Background)
	assert.Equal(t, "project", def.Memory)
	assert.Equal(t, "worktree", def.Isolation)
	assert.Equal(t, "Start here", def.InitialPrompt)
	assert.Equal(t, "purple", def.Color)
}

func TestParseAgentFromJson_UnknownFieldsSilentlyIgnored(t *testing.T) {
	jsonData := []byte(`{
		"description": "A test agent",
		"prompt": "You are a test agent.",
		"unknownField": "should be ignored",
		"anotherUnknown": 123,
		"nestedUnknown": { "a": 1 }
	}`)

	def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
	require.NoError(t, err)
	assert.Equal(t, "A test agent", def.WhenToUse)
	assert.Equal(t, "You are a test agent.", def.SystemPrompt)
}

func TestParseAgentFromJson_ModelInherit(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{`"inherit"`, "inherit"},
		{`"INHERIT"`, "inherit"},
		{`"  InHeRiT  "`, "inherit"},
		{`"sonnet"`, "sonnet"},
		{`"  sonnet  "`, "sonnet"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			jsonData := []byte(`{
				"description": "A test agent",
				"prompt": "You are a test agent.",
				"model": ` + tc.input + `
			}`)
			def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Model)
		})
	}
}

func TestParseAgentFromJson_ColorValidation(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{`"red"`, "red"},
		{`"BLUE"`, "blue"},
		{`"  green  "`, "green"},
		{`"invalid"`, ""},
		{`""`, ""},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			jsonData := []byte(`{
				"description": "A test agent",
				"prompt": "You are a test agent.",
				"color": ` + tc.input + `
			}`)
			def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Color)
		})
	}
}

func TestParseAgentFromJson_Tools(t *testing.T) {
	cases := []struct {
		name     string
		json     string
		expected []string
	}{
		{"nil_missing", `{}`, nil},
		{"empty_array", `{"tools": []}`, []string{}},
		{"specific_tools", `{"tools": ["Read", "Grep"]}`, []string{"Read", "Grep"}},
		{"wildcard", `{"tools": ["*"]}`, nil},
		{"mixed_with_wildcard", `{"tools": ["Read", "*"]}`, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := `{"description": "A test agent", "prompt": "You are a test agent."`
			var jsonData []byte
			if tc.json == `{}` {
				jsonData = []byte(base + `}`)
			} else {
				jsonData = []byte(base + `, ` + tc.json[1:])
			}
			def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Tools)
		})
	}
}

func TestParseAgentFromJson_DisallowedTools(t *testing.T) {
	jsonData := []byte(`{
		"description": "A test agent",
		"prompt": "You are a test agent.",
		"disallowedTools": ["Edit", "Bash"]
	}`)

	def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
	require.NoError(t, err)
	assert.Equal(t, []string{"Edit", "Bash"}, def.DisallowedTools)
}

func TestParseAgentFromJson_Effort(t *testing.T) {
	cases := []struct {
		name     string
		json     string
		expected string
	}{
		{"string_low", `"low"`, "low"},
		{"string_high", `"high"`, "high"},
		{"number", `3`, "3"},
		{"invalid_string", `"invalid"`, ""},
		{"zero", `0`, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jsonData := []byte(`{
				"description": "A test agent",
				"prompt": "You are a test agent.",
				"effort": ` + tc.json + `
			}`)
			def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Effort)
		})
	}
}

func TestParseAgentFromJson_PermissionMode(t *testing.T) {
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
			jsonData := []byte(`{
				"description": "A test agent",
				"prompt": "You are a test agent.",
				"permissionMode": "` + tc.value + `"
			}`)
			def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.PermissionMode)
		})
	}
}

func TestParseAgentFromJson_MaxTurns(t *testing.T) {
	cases := []struct {
		json     string
		expected int
	}{
		{`10`, 10},
		{`0`, 0},
		{`-1`, 0},
	}

	for _, tc := range cases {
		t.Run(tc.json, func(t *testing.T) {
			jsonData := []byte(`{
				"description": "A test agent",
				"prompt": "You are a test agent.",
				"maxTurns": ` + tc.json + `
			}`)
			def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.MaxTurns)
		})
	}
}

func TestParseAgentFromJson_Background(t *testing.T) {
	cases := []struct {
		json     string
		expected bool
	}{
		{`true`, true},
		{`false`, false},
		{`"true"`, true},
		{`"false"`, false},
		{`"yes"`, false},
	}

	for _, tc := range cases {
		t.Run(tc.json, func(t *testing.T) {
			jsonData := []byte(`{
				"description": "A test agent",
				"prompt": "You are a test agent.",
				"background": ` + tc.json + `
			}`)
			def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Background)
		})
	}
}

func TestParseAgentFromJson_Memory(t *testing.T) {
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
			jsonData := []byte(`{
				"description": "A test agent",
				"prompt": "You are a test agent.",
				"memory": "` + tc.value + `"
			}`)
			def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Memory)
		})
	}
}

func TestParseAgentFromJson_Isolation(t *testing.T) {
	cases := []struct {
		value    string
		expected string
	}{
		{"worktree", "worktree"},
		{"remote", ""}, // remote is ant-only, excluded
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			jsonData := []byte(`{
				"description": "A test agent",
				"prompt": "You are a test agent.",
				"isolation": "` + tc.value + `"
			}`)
			def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.Isolation)
		})
	}
}

func TestParseAgentFromJson_InitialPrompt(t *testing.T) {
	jsonData := []byte(`{
		"description": "A test agent",
		"prompt": "You are a test agent.",
		"initialPrompt": "  Hello world  "
	}`)

	def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
	require.NoError(t, err)
	assert.Equal(t, "Hello world", def.InitialPrompt)
}

func TestParseAgentFromJson_MCPServers(t *testing.T) {
	cases := []struct {
		name     string
		json     string
		expected []agent.AgentMCPServerSpec
	}{
		{
			name:     "nil",
			json:     ``,
			expected: nil,
		},
		{
			name:     "empty_array",
			json:     `,"mcpServers": []`,
			expected: nil,
		},
		{
			name:     "reference_only",
			json:     `,"mcpServers": ["slack", "github"]`,
			expected: []agent.AgentMCPServerSpec{{Name: "slack"}, {Name: "github"}},
		},
		{
			name: "inline_definition",
			json: `,"mcpServers": [{"my-server": {"type": "stdio", "command": "npx"}}]`,
			expected: []agent.AgentMCPServerSpec{
				{Name: "my-server", Config: map[string]any{"type": "stdio", "command": "npx"}},
			},
		},
		{
			name:     "mixed",
			json:     `,"mcpServers": ["slack", {"inline": {"type": "stdio", "command": "node"}}]`,
			expected: []agent.AgentMCPServerSpec{{Name: "slack"}, {Name: "inline", Config: map[string]any{"type": "stdio", "command": "node"}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jsonData := []byte(`{
				"description": "A test agent",
				"prompt": "You are a test agent."` + tc.json + `
			}`)
			def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
			require.NoError(t, err)
			assert.Equal(t, tc.expected, def.MCPServers)
		})
	}
}

func TestParseAgentFromJson_Hooks(t *testing.T) {
	cases := []struct {
		name     string
		json     string
		expected bool // whether Hooks should be non-nil
	}{
		{
			name:     "nil",
			json:     ``,
			expected: false,
		},
		{
			name:     "valid_hooks",
			json:     `,"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "echo done"}]}]}`,
			expected: true,
		},
		{
			name:     "invalid_event_ignored",
			json:     `,"hooks": {"UnknownEvent": [{"hooks": [{"type": "command", "command": "echo done"}]}]}`,
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jsonData := []byte(`{
				"description": "A test agent",
				"prompt": "You are a test agent."` + tc.json + `
			}`)
			def, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
			require.NoError(t, err)
			if tc.expected {
				assert.NotNil(t, def.Hooks)
			} else {
				assert.Nil(t, def.Hooks)
			}
		})
	}
}

func TestParseAgentsFromJson_Multiple(t *testing.T) {
	jsonData := []byte(`{
		"agent-one": {
			"description": "First agent",
			"prompt": "You are agent one."
		},
		"agent-two": {
			"description": "Second agent",
			"prompt": "You are agent two.",
			"tools": ["Read"]
		}
	}`)

	defs, err := ParseAgentsFromJson(jsonData, "userSettings")
	require.NoError(t, err)
	require.Len(t, defs, 2)

	// Results may be in any order due to map iteration.
	byName := make(map[string]agent.Definition)
	for _, d := range defs {
		byName[d.AgentType] = d
	}

	assert.Equal(t, "First agent", byName["agent-one"].WhenToUse)
	assert.Equal(t, "You are agent one.", byName["agent-one"].SystemPrompt)
	assert.Equal(t, "userSettings", byName["agent-one"].Source)

	assert.Equal(t, "Second agent", byName["agent-two"].WhenToUse)
	assert.Equal(t, "You are agent two.", byName["agent-two"].SystemPrompt)
	assert.Equal(t, []string{"Read"}, byName["agent-two"].Tools)
}

func TestParseAgentsFromJson_SkipsInvalid(t *testing.T) {
	jsonData := []byte(`{
		"valid-agent": {
			"description": "A valid agent",
			"prompt": "You are valid."
		},
		"invalid-agent": {
			"description": "Missing prompt"
		}
	}`)

	defs, err := ParseAgentsFromJson(jsonData, "flagSettings")
	require.NoError(t, err)
	require.Len(t, defs, 1)
	assert.Equal(t, "valid-agent", defs[0].AgentType)
}

func TestParseAgentsFromJson_EmptyObject(t *testing.T) {
	jsonData := []byte(`{}`)

	defs, err := ParseAgentsFromJson(jsonData, "flagSettings")
	require.NoError(t, err)
	assert.Empty(t, defs)
}

func TestParseAgentsFromJson_InvalidJson(t *testing.T) {
	jsonData := []byte(`not json`)

	_, err := ParseAgentsFromJson(jsonData, "flagSettings")
	require.Error(t, err)
}

func TestParseAgentsFromJson_DefaultSource(t *testing.T) {
	jsonData := []byte(`{
		"agent-one": {
			"description": "First agent",
			"prompt": "You are agent one."
		}
	}`)

	defs, err := ParseAgentsFromJson(jsonData, "")
	require.NoError(t, err)
	require.Len(t, defs, 1)
	assert.Equal(t, "flagSettings", defs[0].Source)
}

func TestParseAgentFromJson_NotAnObject(t *testing.T) {
	jsonData := []byte(`"not an object"`)

	_, err := ParseAgentFromJson("test-agent", jsonData, "flagSettings")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON object")
}

func TestParseToolsFromStrings(t *testing.T) {
	cases := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"nil", nil, nil},
		{"empty", []string{}, []string{}},
		{"single", []string{"Read"}, []string{"Read"}},
		{"wildcard", []string{"*"}, nil},
		{"mixed_wildcard", []string{"Read", "*"}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseToolsFromStrings(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestParseAgentFromJson_WithRawMessage(t *testing.T) {
	// Test that ParseAgentFromJson works with json.RawMessage directly
	var rawMap map[string]json.RawMessage
	jsonData := []byte(`{
		"my-agent": {
			"description": "My agent",
			"prompt": "You are my agent."
		}
	}`)
	err := json.Unmarshal(jsonData, &rawMap)
	require.NoError(t, err)

	def, err := ParseAgentFromJson("my-agent", rawMap["my-agent"], "flagSettings")
	require.NoError(t, err)
	assert.Equal(t, "My agent", def.WhenToUse)
	assert.Equal(t, "You are my agent.", def.SystemPrompt)
}
