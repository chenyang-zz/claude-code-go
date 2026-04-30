package plugin

import (
	"testing"

	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMcpRegistrar_RegisterMcpServers_Success(t *testing.T) {
	registry := mcpregistry.NewServerRegistry()
	registrar := NewMcpRegistrar(registry)

	servers := []*McpServerConfig{
		{
			Name:      "test-server",
			Transport: "stdio",
			Command:   "/usr/bin/test",
			Args:      []string{"--flag"},
			Env:       map[string]string{"KEY": "val"},
		},
		{
			Name:      "remote-server",
			Transport: "sse",
			URL:       "http://localhost:3000",
			Headers:   map[string]string{"Authorization": "Bearer token"},
		},
	}

	registered, errs := registrar.RegisterMcpServers(servers)
	require.Len(t, errs, 0)
	assert.Equal(t, 2, registered)

	// Verify entries were loaded into the registry.
	entries := registry.List()
	require.Len(t, entries, 2)

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	assert.True(t, names["test-server"])
	assert.True(t, names["remote-server"])
}

func TestMcpRegistrar_RegisterMcpServers_Nil(t *testing.T) {
	var registrar *McpRegistrar
	registered, errs := registrar.RegisterMcpServers([]*McpServerConfig{{Name: "s"}})
	assert.Equal(t, 0, registered)
	require.Len(t, errs, 1)
	assert.Equal(t, "registration-error", errs[0].Type)
}

func TestMcpRegistrar_RegisterMcpServers_NilRegistry(t *testing.T) {
	registrar := NewMcpRegistrar(nil)
	registered, errs := registrar.RegisterMcpServers([]*McpServerConfig{{Name: "s"}})
	assert.Equal(t, 0, registered)
	require.Len(t, errs, 1)
	assert.Equal(t, "registration-error", errs[0].Type)
}

func TestMcpRegistrar_RegisterMcpServers_SkipsNilEntries(t *testing.T) {
	registry := mcpregistry.NewServerRegistry()
	registrar := NewMcpRegistrar(registry)

	servers := []*McpServerConfig{
		{Name: "valid", Transport: "stdio"},
		nil,
	}

	registered, errs := registrar.RegisterMcpServers(servers)
	require.Len(t, errs, 0)
	assert.Equal(t, 1, registered)
}

func TestMcpRegistrar_RegisterMcpServers_Empty(t *testing.T) {
	registry := mcpregistry.NewServerRegistry()
	registrar := NewMcpRegistrar(registry)

	registered, errs := registrar.RegisterMcpServers([]*McpServerConfig{})
	require.Len(t, errs, 0)
	assert.Equal(t, 0, registered)
}

func TestToClientServerConfig(t *testing.T) {
	cfg := &McpServerConfig{
		Transport: "ws",
		Command:   "/bin/cmd",
		Args:      []string{"-v"},
		Env:       map[string]string{"X": "y"},
		URL:       "ws://example.com",
		Headers:   map[string]string{"H": "v"},
	}

	clientCfg := toClientServerConfig(cfg)
	assert.Equal(t, "ws", clientCfg.Type)
	assert.Equal(t, "/bin/cmd", clientCfg.Command)
	assert.Equal(t, []string{"-v"}, clientCfg.Args)
	assert.Equal(t, map[string]string{"X": "y"}, clientCfg.Env)
	assert.Equal(t, "ws://example.com", clientCfg.URL)
	assert.Equal(t, map[string]string{"H": "v"}, clientCfg.Headers)
}
