package plugin

import (
	"path/filepath"
	"testing"
)

func TestExtractLspServers_NoLspJson(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	mustMkdirAll(t, pluginPath)

	plugin := &LoadedPlugin{Name: "test-plugin", Path: pluginPath}

	servers, err := ExtractLspServers(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if servers != nil {
		t.Errorf("expected nil, got %v", servers)
	}
}

func TestExtractLspServers_WithLspJson(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	mustMkdirAll(t, pluginPath)
	writeFile(t, filepath.Join(pluginPath, ".lsp.json"), `{
		"typescript": {
			"command": "typescript-language-server",
			"args": ["--stdio"],
			"extensionToLanguage": {".ts": "typescript", ".tsx": "typescriptreact"},
			"transport": "stdio",
			"restartOnCrash": true,
			"maxRestarts": 3
		}
	}`)

	plugin := &LoadedPlugin{Name: "test-plugin", Path: pluginPath}

	servers, err := ExtractLspServers(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}

	s := servers[0]
	if s.Name != "typescript" {
		t.Errorf("expected 'typescript', got %q", s.Name)
	}
	if s.Command != "typescript-language-server" {
		t.Errorf("expected command, got %q", s.Command)
	}
	if len(s.Args) != 1 || s.Args[0] != "--stdio" {
		t.Errorf("expected args ['--stdio'], got %v", s.Args)
	}
	if s.Transport != "stdio" {
		t.Errorf("expected 'stdio', got %q", s.Transport)
	}
	if !s.RestartOnCrash {
		t.Error("expected RestartOnCrash to be true")
	}
	if s.MaxRestarts != 3 {
		t.Errorf("expected MaxRestarts 3, got %d", s.MaxRestarts)
	}
	if s.ExtensionToLanguage[".ts"] != "typescript" {
		t.Errorf("expected .ts → typescript, got %q", s.ExtensionToLanguage[".ts"])
	}
	if s.PluginName != "test-plugin" {
		t.Errorf("expected PluginName 'test-plugin', got %q", s.PluginName)
	}
	if s.Scope != "dynamic" {
		t.Errorf("expected Scope 'dynamic', got %q", s.Scope)
	}
}

func TestExtractLspServers_MultipleServers(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	mustMkdirAll(t, pluginPath)
	writeFile(t, filepath.Join(pluginPath, ".lsp.json"), `{
		"eslint": {
			"command": "vscode-eslint-language-server",
			"args": ["--stdio"],
			"extensionToLanguage": {".js": "javascript"}
		},
		"prettier": {
			"command": "prettier",
			"args": ["--stdio"],
			"extensionToLanguage": {".js": "javascript", ".ts": "typescript"}
		}
	}`)

	plugin := &LoadedPlugin{Name: "test-plugin", Path: pluginPath}

	servers, err := ExtractLspServers(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
}

func TestLoadLspServersFromFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "bad.json")
	writeFile(t, filePath, `{invalid}`)

	_, err := LoadLspServersFromFile(filePath)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidatePathWithinPlugin_Safe(t *testing.T) {
	result := ValidatePathWithinPlugin("/tmp/plugin", "config/servers.json")
	if result == "" {
		t.Error("expected safe path to be allowed")
	}
}

func TestValidatePathWithinPlugin_Traversal(t *testing.T) {
	result := ValidatePathWithinPlugin("/tmp/plugin", "../../etc/passwd")
	if result != "" {
		t.Errorf("expected traversal to be rejected, got %q", result)
	}
}

func TestValidatePathWithinPlugin_Absolute(t *testing.T) {
	result := ValidatePathWithinPlugin("/tmp/plugin", "/etc/passwd")
	if result != "" {
		t.Errorf("expected absolute path to be rejected, got %q", result)
	}
}
