package plugin

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
	"github.com/sheepzhao/claude-code-go/internal/platform/lsp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginRegistrar_RegisterAll(t *testing.T) {
	agentReg := agent.NewInMemoryRegistry()
	cmdReg := command.NewInMemoryRegistry()
	mcpReg := mcpregistry.NewServerRegistry()
	lspMgr := lsp.NewManager()
	hooks := make(hook.HooksConfig)

	registrar := NewPluginRegistrar(agentReg, cmdReg, &hooks, mcpReg, lspMgr)

	result := &RefreshResult{
		Agents: []*AgentDefinition{
			{AgentType: "test-agent", PluginName: "p"},
		},
		Commands: []*PluginCommand{
			{Name: "test-cmd", PluginName: "p"},
		},
		Plugins: []*LoadedPlugin{
			{
				Name: "test-plugin",
				Path: "/plugins/test",
				HooksConfig: &HooksConfig{
					Events: map[string][]HookMatcherEntry{
						"PreToolUse": {
							{
								Matcher: "tool",
								Hooks:   []HookCommand{{Command: "echo"}},
							},
						},
					},
				},
			},
		},
	}

	summary, err := registrar.RegisterAll(result, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.AgentsRegistered)
	assert.Equal(t, 1, summary.CommandsRegistered)
	assert.GreaterOrEqual(t, summary.HooksEventsRegistered, 1)

	_, ok := agentReg.Get("test-agent")
	assert.True(t, ok)

	_, ok = cmdReg.Get("test-cmd")
	assert.True(t, ok)
}

func TestPluginRegistrar_RegisterAll_Nil(t *testing.T) {
	var registrar *PluginRegistrar
	_, err := registrar.RegisterAll(&RefreshResult{}, nil)
	assert.Error(t, err)
}

func TestPluginRegistrar_RegisterAll_EmptyResult(t *testing.T) {
	agentReg := agent.NewInMemoryRegistry()
	registrar := NewPluginRegistrar(agentReg, nil, nil, nil, nil)

	summary, err := registrar.RegisterAll(&RefreshResult{}, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.AgentsRegistered)
	assert.Equal(t, 0, summary.CommandsRegistered)
}

func TestPluginRegistrar_RegisterHooks(t *testing.T) {
	hooks := make(hook.HooksConfig)
	registrar := NewPluginRegistrar(nil, nil, &hooks, nil, nil)

	plugins := []*LoadedPlugin{
		{
			Name: "p",
			Path: "/plugins/p",
			HooksConfig: &HooksConfig{
				Events: map[string][]HookMatcherEntry{
					"PreToolUse": {
						{
							Matcher: "tool",
							Hooks:   []HookCommand{{Command: "echo"}},
						},
					},
				},
			},
		},
	}

	count, err := registrar.RegisterHooks(plugins, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1)
	assert.Len(t, hooks[hook.EventPreToolUse], 1)
}

func TestPluginRegistrar_RegisterHooks_NilConfig(t *testing.T) {
	registrar := NewPluginRegistrar(nil, nil, nil, nil, nil)
	count, err := registrar.RegisterHooks(nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestPluginRegistrar_RegisterMcpServers(t *testing.T) {
	mcpReg := mcpregistry.NewServerRegistry()
	registrar := NewPluginRegistrar(nil, nil, nil, mcpReg, nil)

	servers := []*McpServerConfig{
		{Name: "s1", Transport: "stdio"},
	}

	count, err := registrar.RegisterMcpServers(servers)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPluginRegistrar_RegisterMcpServers_NilRegistry(t *testing.T) {
	registrar := NewPluginRegistrar(nil, nil, nil, nil, nil)
	count, err := registrar.RegisterMcpServers([]*McpServerConfig{{Name: "s1"}})
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestPluginRegistrar_RegisterLspServers(t *testing.T) {
	lspMgr := lsp.NewManager()
	registrar := NewPluginRegistrar(nil, nil, nil, nil, lspMgr)

	servers := []*LspServerConfig{
		{Name: "s1", Command: "/bin/ls", ExtensionToLanguage: map[string]string{".txt": "text"}},
	}

	count, err := registrar.RegisterLspServers(servers)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPluginRegistrar_RegisterLspServers_NilManager(t *testing.T) {
	registrar := NewPluginRegistrar(nil, nil, nil, nil, nil)
	count, err := registrar.RegisterLspServers([]*LspServerConfig{{Name: "s1"}})
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestRegistrationSummary_FormatSummary(t *testing.T) {
	s := &RegistrationSummary{
		AgentsRegistered:     2,
		CommandsRegistered:   3,
		McpServersLoaded:     1,
		LspServersRegistered: 1,
		Errors:               []string{"err1"},
	}

	out := s.FormatSummary()
	assert.Contains(t, out, "2 agent(s)")
	assert.Contains(t, out, "3 command(s)")
	assert.Contains(t, out, "1 MCP server(s)")
	assert.Contains(t, out, "1 LSP server(s)")
	assert.Contains(t, out, "1 error(s)")
	assert.Contains(t, out, "err1")
}

func TestRegistrationSummary_FormatSummary_Nil(t *testing.T) {
	var s *RegistrationSummary
	assert.Equal(t, "No plugin registration performed.", s.FormatSummary())
}
