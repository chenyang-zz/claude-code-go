package plugin

import (
	"path/filepath"
	"testing"
)

func TestExtractMcpServers_NoMcpJson(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	mustMkdirAll(t, pluginPath)

	plugin := &LoadedPlugin{
		Name:   "test-plugin",
		Path:   pluginPath,
		Source: PluginSource{Type: SourceTypePath, Value: "/tmp/test-plugin"},
	}

	servers, err := ExtractMcpServers(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if servers != nil {
		t.Errorf("expected nil, got %v", servers)
	}
}

func TestExtractMcpServers_WithMcpJson_WrapperFormat(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	mustMkdirAll(t, pluginPath)
	writeFile(t, filepath.Join(pluginPath, ".mcp.json"), `{
		"mcpServers": {
			"my-server": {
				"transport": "stdio",
				"command": "node",
				"args": ["server.js"],
				"env": {"NODE_ENV": "production"}
			}
		}
	}`)

	plugin := &LoadedPlugin{
		Name:   "test-plugin",
		Path:   pluginPath,
		Source: PluginSource{Type: SourceTypePath, Value: "test-plugin@local"},
	}

	servers, err := ExtractMcpServers(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}

	s := servers[0]
	if s.Name != "my-server" {
		t.Errorf("expected 'my-server', got %q", s.Name)
	}
	if s.Transport != "stdio" {
		t.Errorf("expected 'stdio', got %q", s.Transport)
	}
	if s.Command != "node" {
		t.Errorf("expected 'node', got %q", s.Command)
	}
	if len(s.Args) != 1 || s.Args[0] != "server.js" {
		t.Errorf("expected args ['server.js'], got %v", s.Args)
	}
	if s.Env["NODE_ENV"] != "production" {
		t.Errorf("expected NODE_ENV=production, got %q", s.Env["NODE_ENV"])
	}
	if s.PluginName != "test-plugin" {
		t.Errorf("expected PluginName 'test-plugin', got %q", s.PluginName)
	}
	if s.PluginSource != "test-plugin" {
		t.Errorf("expected PluginSource 'test-plugin', got %q", s.PluginSource)
	}
	if s.Scope != "dynamic" {
		t.Errorf("expected Scope 'dynamic', got %q", s.Scope)
	}
}

func TestExtractMcpServers_WithMcpJson_DirectFormat(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	mustMkdirAll(t, pluginPath)
	writeFile(t, filepath.Join(pluginPath, ".mcp.json"), `{
		"direct-server": {
			"transport": "sse",
			"url": "https://example.com/mcp"
		}
	}`)

	plugin := &LoadedPlugin{
		Name:   "test-plugin",
		Path:   pluginPath,
		Source: PluginSource{Type: SourceTypePath, Value: "local"},
	}

	servers, err := ExtractMcpServers(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Name != "direct-server" {
		t.Errorf("expected 'direct-server', got %q", servers[0].Name)
	}
	if servers[0].Transport != "sse" {
		t.Errorf("expected 'sse', got %q", servers[0].Transport)
	}
}

func TestExtractMcpServers_MultipleServers(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	mustMkdirAll(t, pluginPath)
	writeFile(t, filepath.Join(pluginPath, ".mcp.json"), `{
		"mcpServers": {
			"server-a": {"transport": "stdio", "command": "a"},
			"server-b": {"transport": "stdio", "command": "b"},
			"server-c": {"transport": "http", "url": "http://c"}
		}
	}`)

	plugin := &LoadedPlugin{
		Name:   "test-plugin",
		Path:   pluginPath,
		Source: PluginSource{Type: SourceTypePath, Value: "local"},
	}

	servers, err := ExtractMcpServers(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(servers) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(servers))
	}
}

func TestLoadMcpServersFromFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "bad.json")
	writeFile(t, filePath, `{invalid}`)

	_, err := LoadMcpServersFromFile(filePath)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadMcpServersFromFile_SkipsInvalidEntries(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "mixed.json")
	writeFile(t, filePath, `{
		"valid-server": {"transport": "stdio", "command": "ok"},
		"bad-server": "not_an_object"
	}`)

	servers, err := LoadMcpServersFromFile(filePath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 valid server, got %d", len(servers))
	}
	if servers[0].Name != "valid-server" {
		t.Errorf("expected 'valid-server', got %q", servers[0].Name)
	}
}
