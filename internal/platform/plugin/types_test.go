package plugin

import (
	"encoding/json"
	"testing"
)

func TestPluginManifestJSONRoundTrip(t *testing.T) {
	manifest := PluginManifest{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "A test plugin for unit testing",
		Author: &PluginAuthor{
			Name:  "Test Author",
			Email: "test@example.com",
			URL:   "https://example.com",
		},
		Homepage:   "https://plugin.example.com",
		Repository: "https://github.com/test/plugin",
		License:    "MIT",
		Keywords:   []string{"test", "utility"},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	var parsed PluginManifest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal manifest: %v", err)
	}

	if parsed.Name != manifest.Name {
		t.Errorf("expected name %q, got %q", manifest.Name, parsed.Name)
	}
	if parsed.Version != manifest.Version {
		t.Errorf("expected version %q, got %q", manifest.Version, parsed.Version)
	}
	if parsed.Author == nil {
		t.Fatal("expected author to be non-nil")
	}
	if parsed.Author.Name != manifest.Author.Name {
		t.Errorf("expected author name %q, got %q", manifest.Author.Name, parsed.Author.Name)
	}
}

func TestPluginManifestMinimal(t *testing.T) {
	// Only name is required.
	data := []byte(`{"name": "minimal-plugin"}`)
	var m PluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal minimal manifest: %v", err)
	}
	if m.Name != "minimal-plugin" {
		t.Errorf("expected name %q, got %q", "minimal-plugin", m.Name)
	}
	if m.Version != "" {
		t.Errorf("expected empty version, got %q", m.Version)
	}
	if m.Author != nil {
		t.Error("expected nil author for minimal manifest")
	}
}

func TestPluginManifestOmitsUnknownFields(t *testing.T) {
	data := []byte(`{"name": "test", "unknown_field": "should be ignored", "another_unknown": 42}`)
	var m PluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal manifest with unknown fields: %v", err)
	}
	if m.Name != "test" {
		t.Errorf("expected name %q, got %q", "test", m.Name)
	}
}

func TestPluginSourceJSON(t *testing.T) {
	src := PluginSource{
		Type:    SourceTypeGitHub,
		Value:   "anthropics/claude-plugins",
		Version: "main",
	}

	data, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("failed to marshal source: %v", err)
	}

	var parsed PluginSource
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal source: %v", err)
	}
	if parsed.Type != SourceTypeGitHub {
		t.Errorf("expected type %q, got %q", SourceTypeGitHub, parsed.Type)
	}
	if parsed.Value != src.Value {
		t.Errorf("expected value %q, got %q", src.Value, parsed.Value)
	}
}

func TestPluginErrorImplementsError(t *testing.T) {
	e := &PluginError{
		Type:    "test-error",
		Source:  "test-source",
		Plugin:  "test-plugin",
		Message: "something went wrong",
	}

	var _ error = e // compile-time assertion

	if e.Error() != "something went wrong" {
		t.Errorf("expected error message %q, got %q", "something went wrong", e.Error())
	}
}

func TestPluginErrorJSON(t *testing.T) {
	e := &PluginError{
		Type:    "manifest-parse-error",
		Source:  "marketplace-test",
		Plugin:  "bad-plugin",
		Message: "invalid JSON in manifest",
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("failed to marshal error: %v", err)
	}

	var parsed PluginError
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}
	if parsed.Type != e.Type {
		t.Errorf("expected type %q, got %q", e.Type, parsed.Type)
	}
	if parsed.Message != e.Message {
		t.Errorf("expected message %q, got %q", e.Message, parsed.Message)
	}
}

func TestPluginLoadResultEmpty(t *testing.T) {
	result := PluginLoadResult{}
	if len(result.Enabled) != 0 {
		t.Errorf("expected 0 enabled, got %d", len(result.Enabled))
	}
	if len(result.Disabled) != 0 {
		t.Errorf("expected 0 disabled, got %d", len(result.Disabled))
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
}

func TestPluginLoadResultWithContent(t *testing.T) {
	result := PluginLoadResult{
		Enabled:  []*LoadedPlugin{{Name: "enabled-plugin", Enabled: true}},
		Disabled: []*LoadedPlugin{{Name: "disabled-plugin", Enabled: false}},
		Errors:   []*PluginError{{Type: "test-error", Message: "test"}},
	}
	if len(result.Enabled) != 1 {
		t.Errorf("expected 1 enabled, got %d", len(result.Enabled))
	}
	if len(result.Disabled) != 1 {
		t.Errorf("expected 1 disabled, got %d", len(result.Disabled))
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
}

func TestLoadedPluginStruct(t *testing.T) {
	p := &LoadedPlugin{
		Name:   "my-plugin",
		Path:   "/path/to/plugin",
		Source: PluginSource{Type: SourceTypePath, Value: "/path/to/plugin"},
		Manifest: PluginManifest{
			Name:        "my-plugin",
			Version:     "2.0.0",
			Description: "My test plugin",
		},
		Enabled:   true,
		IsBuiltin: false,
	}

	if p.Name != "my-plugin" {
		t.Errorf("expected name %q, got %q", "my-plugin", p.Name)
	}
	if p.Manifest.Name != "my-plugin" {
		t.Errorf("expected manifest name %q, got %q", "my-plugin", p.Manifest.Name)
	}
	if !p.Enabled {
		t.Error("expected plugin to be enabled")
	}
	if p.IsBuiltin {
		t.Error("expected plugin to not be builtin")
	}
}

func TestPluginSourceTypeConstants(t *testing.T) {
	tests := []struct {
		name string
		val  PluginSourceType
		want string
	}{
		{"path", SourceTypePath, "path"},
		{"git", SourceTypeGit, "git"},
		{"github", SourceTypeGitHub, "github"},
		{"npm", SourceTypeNPM, "npm"},
		{"builtin", SourceTypeBuiltin, "builtin"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.val) != tc.want {
				t.Errorf("expected %q, got %q", tc.want, string(tc.val))
			}
		})
	}
}
