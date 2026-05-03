package mcpb

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// M4-1: source.go tests
// =============================================================================

func TestIsMcpbSource(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{"myplugin.mcpb", true},
		{"server.dxt", true},
		{"plugin-v1.0.mcpb", true},
		{"plugin.json", false},
		{"plugin.mcp", false},
		{"plugin.mcp.json", false},
		{"mcpb", false},
		{"file.dxt.zip", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsMcpbSource(tt.source)
		if got != tt.want {
			t.Errorf("IsMcpbSource(%q) = %v, want %v", tt.source, got, tt.want)
		}
	}
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{"https://example.com/plugin.mcpb", true},
		{"http://localhost:8080/file.dxt", true},
		{"file.mcpb", false},
		{"/path/to/file.mcpb", false},
		{"./relative/file.dxt", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsURL(tt.source)
		if got != tt.want {
			t.Errorf("IsURL(%q) = %v, want %v", tt.source, got, tt.want)
		}
	}
}

// =============================================================================
// M4-1: manifest.go tests
// =============================================================================

func TestParseManifestFromBytes_Valid(t *testing.T) {
	manifest := McpbManifest{
		Name:    "test-server",
		Version: "1.0.0",
		Author:  McpbManifestAuthor{Name: "Test Author"},
		Server: &McpbManifestServer{
			Command:   "node",
			Args:      []string{"server.js"},
			Transport: "stdio",
			Env:       map[string]string{"NODE_ENV": "production"},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseManifestFromBytes(data)
	if err != nil {
		t.Fatalf("ParseManifestFromBytes failed: %v", err)
	}
	if parsed.Name != "test-server" {
		t.Errorf("Name = %q, want %q", parsed.Name, "test-server")
	}
	if parsed.Server.Command != "node" {
		t.Errorf("Command = %q, want %q", parsed.Server.Command, "node")
	}
}

func TestParseManifestFromBytes_MissingName(t *testing.T) {
	data := []byte(`{"server": {"command": "node"}}`)
	_, err := ParseManifestFromBytes(data)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseManifestFromBytes_MissingServer(t *testing.T) {
	data := []byte(`{"name": "test"}`)
	_, err := ParseManifestFromBytes(data)
	if err == nil {
		t.Fatal("expected error for missing server")
	}
}

func TestParseManifestFromBytes_NoCommandOrURL(t *testing.T) {
	data := []byte(`{"name": "test", "server": {"transport": "stdio"}}`)
	_, err := ParseManifestFromBytes(data)
	if err == nil {
		t.Fatal("expected error for server without command or url")
	}
}

func TestParseManifestFromBytes_InvalidJSON(t *testing.T) {
	_, err := ParseManifestFromBytes([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestBuildMcpServerConfig(t *testing.T) {
	manifest := &McpbManifest{
		Name:    "test-server",
		Version: "1.0.0",
		Server: &McpbManifestServer{
			Command:   "python",
			Args:      []string{"-m", "mcp_server", "--port", "${user_config.PORT}"},
			Transport: "stdio",
			Env:       map[string]string{"DEBUG": "${user_config.DEBUG}"},
		},
	}

	userConfig := UserConfigValues{
		"PORT":  "8080",
		"DEBUG": "true",
	}

	config, err := BuildMcpServerConfig(manifest, "/extracted", userConfig)
	if err != nil {
		t.Fatalf("BuildMcpServerConfig failed: %v", err)
	}
	if config.Name != "test-server" {
		t.Errorf("Name = %q, want %q", config.Name, "test-server")
	}
	if config.Transport != "stdio" {
		t.Errorf("Transport = %q, want %q", config.Transport, "stdio")
	}
	// Check user config substitution in args.
	// Original: ["-m", "mcp_server", "--port", "${user_config.PORT}"]
	// After substitution: ["-m", "mcp_server", "--port", "8080"]
	if config.Args[2] != "--port" {
		t.Errorf("Args[2] = %q, want %q", config.Args[2], "--port")
	}
	if config.Args[3] != "8080" {
		t.Errorf("PORT substitution: got %q, want %q", config.Args[3], "8080")
	}
	if config.Env["DEBUG"] != "true" {
		t.Errorf("DEBUG substitution: got %q, want %q", config.Env["DEBUG"], "true")
	}
}

func TestBuildMcpServerConfig_DefaultTransport(t *testing.T) {
	manifest := &McpbManifest{
		Name: "test",
		Server: &McpbManifestServer{
			Command: "echo",
		},
	}
	config, err := BuildMcpServerConfig(manifest, "/tmp", nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.Transport != "stdio" {
		t.Errorf("default transport = %q, want %q", config.Transport, "stdio")
	}
}

func TestBuildMcpServerConfig_URLTransport(t *testing.T) {
	manifest := &McpbManifest{
		Name: "web-server",
		Server: &McpbManifestServer{
			URL:       "https://example.com/mcp",
			Transport: "sse",
		},
	}
	config, err := BuildMcpServerConfig(manifest, "/tmp", nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.URL != "https://example.com/mcp" {
		t.Errorf("URL = %q", config.URL)
	}
}

// =============================================================================
// M4-1: config.go tests
// =============================================================================

func TestValidateUserConfig_Valid(t *testing.T) {
	schema := map[string]McpbConfigOption{
		"api_key":   {Type: "string", Required: true, Title: "API Key", Sensitive: true},
		"port":      {Type: "number", Required: false, Min: float64Ptr(1), Max: float64Ptr(65535)},
		"debug":     {Type: "boolean", Required: false},
		"data_dir":  {Type: "directory", Required: false},
	}
	values := UserConfigValues{
		"api_key":  "secret123",
		"port":     float64(8080),
		"debug":    true,
		"data_dir": "/tmp/data",
	}
	errors := ValidateUserConfig(values, schema)
	if len(errors) > 0 {
		t.Errorf("unexpected validation errors: %v", errors)
	}
}

func TestValidateUserConfig_MissingRequired(t *testing.T) {
	schema := map[string]McpbConfigOption{
		"token": {Type: "string", Required: true, Title: "Token"},
	}
	errors := ValidateUserConfig(UserConfigValues{}, schema)
	if len(errors) == 0 {
		t.Fatal("expected validation errors for missing required field")
	}
}

func TestValidateUserConfig_WrongType(t *testing.T) {
	schema := map[string]McpbConfigOption{
		"port": {Type: "number", Required: true},
	}
	errors := ValidateUserConfig(UserConfigValues{"port": "not-a-number"}, schema)
	if len(errors) == 0 {
		t.Fatal("expected validation error for wrong type")
	}
}

func TestValidateUserConfig_Boolean(t *testing.T) {
	schema := map[string]McpbConfigOption{
		"enabled": {Type: "boolean", Required: true},
	}
	errors := ValidateUserConfig(UserConfigValues{"enabled": "yes"}, schema)
	if len(errors) == 0 {
		t.Fatal("expected validation error for non-boolean")
	}

	errors = ValidateUserConfig(UserConfigValues{"enabled": true}, schema)
	if len(errors) > 0 {
		t.Errorf("unexpected errors for valid boolean: %v", errors)
	}
}

func TestValidateUserConfig_NumberRange(t *testing.T) {
	schema := map[string]McpbConfigOption{
		"count": {Type: "number", Min: float64Ptr(0), Max: float64Ptr(100)},
	}
	errors := ValidateUserConfig(UserConfigValues{"count": float64(150)}, schema)
	if len(errors) == 0 {
		t.Fatal("expected validation error for out-of-range number")
	}

	errors = ValidateUserConfig(UserConfigValues{"count": float64(50)}, schema)
	if len(errors) > 0 {
		t.Errorf("unexpected errors for in-range number: %v", errors)
	}
}

func TestValidateUserConfig_StringArray(t *testing.T) {
	schema := map[string]McpbConfigOption{
		"tags": {Type: "string", Multiple: true},
	}
	errors := ValidateUserConfig(UserConfigValues{"tags": []any{"a", "b"}}, schema)
	if len(errors) > 0 {
		t.Errorf("unexpected errors for string array: %v", errors)
	}

	errors = ValidateUserConfig(UserConfigValues{"tags": []any{1, 2}}, schema)
	if len(errors) == 0 {
		t.Fatal("expected error for non-string array elements")
	}
}

func TestValidateUserConfig_OptionalNotProvided(t *testing.T) {
	schema := map[string]McpbConfigOption{
		"name":    {Type: "string", Required: true},
		"timeout": {Type: "number", Required: false},
	}
	errors := ValidateUserConfig(UserConfigValues{"name": "test"}, schema)
	if len(errors) > 0 {
		t.Errorf("unexpected errors when optional field not provided: %v", errors)
	}
}

func TestServerSecretsKey(t *testing.T) {
	key := serverSecretsKey("my-plugin", "my-server")
	expected := "my-plugin/my-server"
	if key != expected {
		t.Errorf("serverSecretsKey = %q, want %q", key, expected)
	}
}

func TestLoadMcpServerUserConfig(t *testing.T) {
	nonSensitive := UserConfigValues{"host": "localhost", "port": 8080}
	sensitiveLoader := func(key string) UserConfigValues {
		if key == "my-plugin/my-server" {
			return UserConfigValues{"password": "secret123"}
		}
		return nil
	}

	result := LoadMcpServerUserConfig("my-plugin", "my-server", nonSensitive, sensitiveLoader)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["host"] != "localhost" {
		t.Errorf("host = %v", result["host"])
	}
	if result["password"] != "secret123" {
		t.Errorf("password = %v", result["password"])
	}
}

func TestLoadMcpServerUserConfig_NilSources(t *testing.T) {
	result := LoadMcpServerUserConfig("p", "s", nil, nil)
	if result != nil {
		t.Fatal("expected nil result when both sources are nil")
	}
}

// =============================================================================
// M4-1: cache.go tests
// =============================================================================

func TestGetMcpbCacheDir(t *testing.T) {
	dir := GetMcpbCacheDir("/path/to/plugin")
	expected := filepath.Join("/path/to/plugin", ".mcpb-cache")
	if dir != expected {
		t.Errorf("GetMcpbCacheDir = %q, want %q", dir, expected)
	}
}

func TestLoadSaveCacheMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, ".mcpb-cache")
	source := "test-plugin.mcpb"

	// Load non-existent metadata should return nil.
	if meta := LoadCacheMetadata(cacheDir, source); meta != nil {
		t.Fatal("expected nil for non-existent cache metadata")
	}

	// Save metadata.
	meta := &McpbCacheMetadata{
		Source:        source,
		ContentHash:   "abc123def456",
		ExtractedPath: filepath.Join(cacheDir, "abc123def456"),
		CachedAt:      "2026-05-03T10:00:00Z",
		LastChecked:   "2026-05-03T10:00:00Z",
	}
	if err := SaveCacheMetadata(cacheDir, source, meta); err != nil {
		t.Fatalf("SaveCacheMetadata failed: %v", err)
	}

	// Load should return matching metadata.
	loaded := LoadCacheMetadata(cacheDir, source)
	if loaded == nil {
		t.Fatal("expected metadata after save")
	}
	if loaded.ContentHash != "abc123def456" {
		t.Errorf("ContentHash = %q, want %q", loaded.ContentHash, "abc123def456")
	}
}

func TestCheckMcpbChanged_NoCache(t *testing.T) {
	// With no cache metadata, should return true (needs reload).
	changed := CheckMcpbChanged("test.mcpb", "/nonexistent/plugin")
	if !changed {
		t.Fatal("expected changed=true when no cache exists")
	}
}

func TestCheckMcpbChanged_MissingExtractDir(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, ".mcpb-cache")
	source := "test.mcpb"

	meta := &McpbCacheMetadata{
		Source:        source,
		ContentHash:   "abc",
		ExtractedPath: filepath.Join(cacheDir, "nonexistent-dir"),
		CachedAt:      "2026-05-03T10:00:00Z",
		LastChecked:   "2026-05-03T10:00:00Z",
	}
	if err := SaveCacheMetadata(cacheDir, source, meta); err != nil {
		t.Fatal(err)
	}

	// Extraction directory doesn't exist — should return true.
	changed := CheckMcpbChanged(source, tmpDir)
	if !changed {
		t.Fatal("expected changed=true when extraction dir is missing")
	}
}

// =============================================================================
// M4-1: download.go tests
// =============================================================================

func TestContentHash(t *testing.T) {
	data := []byte("hello world")
	hash := contentHash(data)
	if len(hash) != 16 {
		t.Errorf("contentHash length = %d, want 16", len(hash))
	}
	// Same data should produce same hash.
	if contentHash(data) != hash {
		t.Error("contentHash should be deterministic")
	}
	// Different data should produce different hash.
	if contentHash([]byte("different")) == hash {
		t.Error("different data should produce different hash")
	}
}

// =============================================================================
// M4-1: extract.go tests
// =============================================================================

func TestExtractMcpb(t *testing.T) {
	// Create a small in-memory ZIP file.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Add manifest.json.
	manifestWriter, _ := zw.Create("manifest.json")
	manifestWriter.Write([]byte(`{"name":"test","server":{"command":"echo"}}`))

	// Add a binary file.
	binWriter, _ := zw.Create("bin/server")
	binWriter.Write([]byte("binary content"))

	zw.Close()

	tmpDir := t.TempDir()
	extractPath := filepath.Join(tmpDir, "extracted")

	err := ExtractMcpb(buf.Bytes(), extractPath, nil)
	if err != nil {
		t.Fatalf("ExtractMcpb failed: %v", err)
	}

	// Verify manifest.json was extracted.
	manifestData, err := os.ReadFile(filepath.Join(extractPath, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest.json not found after extraction: %v", err)
	}
	if string(manifestData) != `{"name":"test","server":{"command":"echo"}}` {
		t.Errorf("manifest content mismatch: %s", string(manifestData))
	}

	// Verify binary file was extracted.
	binData, err := os.ReadFile(filepath.Join(extractPath, "bin/server"))
	if err != nil {
		t.Fatalf("bin/server not found after extraction: %v", err)
	}
	if string(binData) != "binary content" {
		t.Errorf("binary content mismatch")
	}
}

func TestExtractMcpb_InvalidZip(t *testing.T) {
	err := ExtractMcpb([]byte("not a zip file"), t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected error for invalid ZIP data")
	}
}

func TestIsTextFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"file.json", true},
		{"file.js", true},
		{"file.ts", true},
		{"file.txt", true},
		{"file.md", true},
		{"file.yml", true},
		{"file.yaml", true},
		{"file.toml", true},
		{"file.xml", true},
		{"file.html", true},
		{"file.css", true},
		{"server", false},
		{"binary.exe", false},
		{"image.png", false},
		{"archive.zip", false},
	}
	for _, tt := range tests {
		got := isTextFile(tt.name)
		if got != tt.want {
			t.Errorf("isTextFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// =============================================================================
// M4-1: loader.go integration tests
// =============================================================================

func TestLoadMcpbConfig_LocalFile(t *testing.T) {
	// Create a minimal MCPB ZIP file.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("manifest.json")
	w.Write([]byte(`{"name":"local-test","version":"1.0","author":{"name":"Test"},"server":{"command":"echo","args":["hello"]}}`))
	zw.Close()

	// Write the ZIP to a temp file as .mcpb.
	tmpDir := t.TempDir()
	mcpbPath := filepath.Join(tmpDir, "test.mcpb")
	if err := os.WriteFile(mcpbPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	cl := &ConfigLoader{
		PluginPath: tmpDir,
		PluginID:   "test-plugin",
	}

	result, needsConfig, err := LoadMcpbConfig("test.mcpb", cl, nil, nil)
	if err != nil {
		t.Fatalf("LoadMcpbConfig failed: %v", err)
	}
	if needsConfig != nil {
		t.Fatal("unexpected needs-config result")
	}
	if result == nil {
		t.Fatal("expected load result")
	}
	if result.Manifest.Name != "local-test" {
		t.Errorf("manifest name = %q", result.Manifest.Name)
	}
	if result.McpConfig.Name != "local-test" {
		t.Errorf("config name = %q", result.McpConfig.Name)
	}
	if result.ContentHash == "" {
		t.Error("expected non-empty content hash")
	}
	if result.ExtractedPath == "" {
		t.Error("expected non-empty extraction path")
	}
}

func TestLoadMcpbConfig_WithUserConfig(t *testing.T) {
	// Create MCPB ZIP with user_config.
	manifest := McpbManifest{
		Name:    "config-server",
		Version: "1.0",
		Author:  McpbManifestAuthor{Name: "Test"},
		Server: &McpbManifestServer{
			Command: "node",
			Args:    []string{"server.js", "--port", "${user_config.PORT}"},
		},
		UserConfig: map[string]McpbConfigOption{
			"PORT": {Type: "number", Required: true, Title: "Port", Min: float64Ptr(1), Max: float64Ptr(65535)},
		},
	}
	manifestData, _ := json.Marshal(manifest)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("manifest.json")
	w.Write(manifestData)
	zw.Close()

	tmpDir := t.TempDir()
	mcpbPath := filepath.Join(tmpDir, "config.mcpb")
	if err := os.WriteFile(mcpbPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	cl := &ConfigLoader{
		PluginPath: tmpDir,
		PluginID:   "test-plugin",
	}

	// First load without config — should get needs-config result.
	_, needsConfig, err := LoadMcpbConfig("config.mcpb", cl, nil, nil)
	if err != nil {
		t.Fatalf("LoadMcpbConfig failed: %v", err)
	}
	if needsConfig == nil {
		t.Fatal("expected needs-config result for server with user_config")
	}
	if needsConfig.Status != "needs-config" {
		t.Errorf("status = %q, want %q", needsConfig.Status, "needs-config")
	}
	if len(needsConfig.ValidationErrors) == 0 {
		t.Error("expected validation errors for missing required PORT")
	}

	// Now load with config — should succeed.
	result, needsConfig, err := LoadMcpbConfig("config.mcpb", cl, nil,
		UserConfigValues{"PORT": float64(3000)})
	if err != nil {
		t.Fatalf("LoadMcpbConfig with config failed: %v", err)
	}
	if needsConfig != nil {
		t.Fatalf("unexpected needs-config: %v", needsConfig.ValidationErrors)
	}
	if result == nil {
		t.Fatal("expected load result")
	}
	if result.McpConfig.Name != "config-server" {
		t.Errorf("name = %q", result.McpConfig.Name)
	}
}

func TestLoadMcpbConfig_CacheHit(t *testing.T) {
	// Create MCPB ZIP.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("manifest.json")
	w.Write([]byte(`{"name":"cached-server","server":{"command":"echo"}}`))
	zw.Close()

	tmpDir := t.TempDir()
	mcpbPath := filepath.Join(tmpDir, "cached.mcpb")
	if err := os.WriteFile(mcpbPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	cl := &ConfigLoader{
		PluginPath: tmpDir,
		PluginID:   "test-plugin",
	}

	// First load.
	result1, _, err := LoadMcpbConfig("cached.mcpb", cl, nil, nil)
	if err != nil {
		t.Fatalf("first LoadMcpbConfig failed: %v", err)
	}

	// Second load should hit cache (same content hash).
	result2, _, err := LoadMcpbConfig("cached.mcpb", cl, nil, nil)
	if err != nil {
		t.Fatalf("second LoadMcpbConfig failed: %v", err)
	}

	if result1.ContentHash != result2.ContentHash {
		t.Error("cache hit should produce same content hash")
	}
}

func TestLoadMcpbConfig_LocalFileNotFound(t *testing.T) {
	cl := &ConfigLoader{
		PluginPath: t.TempDir(),
		PluginID:   "test",
	}
	_, _, err := LoadMcpbConfig("nonexistent.mcpb", cl, nil, nil)
	if err == nil {
		t.Fatal("expected error for non-existent local file")
	}
}

// =============================================================================
// M4-1: helper
// =============================================================================

func float64Ptr(f float64) *float64 {
	return &f
}
