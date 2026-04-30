package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestPluginJSON(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"name": "test-plugin",
		"version": "1.0.0",
		"description": "A test plugin",
		"author": {
			"name": "Test Author",
			"email": "test@example.com"
		},
		"license": "MIT",
		"keywords": ["test", "example"]
	}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write plugin.json: %v", err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "test-plugin" {
		t.Errorf("expected name %q, got %q", "test-plugin", m.Name)
	}
	if m.Version != "1.0.0" {
		t.Errorf("expected version %q, got %q", "1.0.0", m.Version)
	}
	if m.Description != "A test plugin" {
		t.Errorf("expected description %q, got %q", "A test plugin", m.Description)
	}
	if m.Author == nil {
		t.Fatal("expected author to be non-nil")
	}
	if m.Author.Name != "Test Author" {
		t.Errorf("expected author name %q, got %q", "Test Author", m.Author.Name)
	}
	if m.License != "MIT" {
		t.Errorf("expected license %q, got %q", "MIT", m.License)
	}
	if len(m.Keywords) != 2 {
		t.Errorf("expected 2 keywords, got %d", len(m.Keywords))
	}
}

func TestLoadManifestPackageJSONFallback(t *testing.T) {
	dir := t.TempDir()
	content := `{"name": "pkg-json-plugin", "version": "2.0.0"}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "pkg-json-plugin" {
		t.Errorf("expected name %q, got %q", "pkg-json-plugin", m.Name)
	}
}

func TestLoadManifestPluginJSONPreferred(t *testing.T) {
	dir := t.TempDir()
	// plugin.json takes priority.
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name":"from-plugin-json"}`), 0o644); err != nil {
		t.Fatalf("failed to write plugin.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"from-package-json"}`), 0o644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "from-plugin-json" {
		t.Errorf("expected plugin.json to take precedence, got %q", m.Name)
	}
}

func TestLoadManifestNoManifest(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadManifest(dir)
	if err == nil {
		t.Error("expected error for dir with no manifest")
	}
}

func TestLoadManifestMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{invalid json`), 0o644); err != nil {
		t.Fatalf("failed to write malformed plugin.json: %v", err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestLoadManifestEmptyName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name": ""}`), 0o644); err != nil {
		t.Fatalf("failed to write plugin.json: %v", err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestLoadManifestNameWithSpaces(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name": "my plugin"}`), 0o644); err != nil {
		t.Fatalf("failed to write plugin.json: %v", err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Error("expected error for name with spaces")
	}
}

func TestValidateManifestNil(t *testing.T) {
	err := ValidateManifest(nil)
	if err == nil {
		t.Error("expected error for nil manifest")
	}
}

func TestValidateManifestValid(t *testing.T) {
	manifest := &PluginManifest{
		Name:        "my-plugin",
		Version:     "1.0.0",
		Description: "A valid plugin",
	}
	if err := ValidateManifest(manifest); err != nil {
		t.Errorf("unexpected error for valid manifest: %v", err)
	}
}

func TestValidateManifestEmptyName(t *testing.T) {
	tests := []struct {
		name     string
		manifest *PluginManifest
	}{
		{"empty string", &PluginManifest{Name: ""}},
		{"whitespace only", &PluginManifest{Name: "   "}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateManifest(tc.manifest)
			if err == nil {
				t.Error("expected error for manifest with empty/whitespace name")
			}
		})
	}
}

func TestValidateManifestNameWithSpaces(t *testing.T) {
	manifest := &PluginManifest{Name: "bad name"}
	err := ValidateManifest(manifest)
	if err == nil {
		t.Error("expected error for name with spaces")
	}
}

func TestValidateManifestKebabCase(t *testing.T) {
	manifest := &PluginManifest{Name: "my-awesome-plugin"}
	if err := ValidateManifest(manifest); err != nil {
		t.Errorf("unexpected error for kebab-case name: %v", err)
	}
}

func TestFindManifestFilePluginJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("failed to write plugin.json: %v", err)
	}

	path := findManifestFile(dir)
	if path != filepath.Join(dir, "plugin.json") {
		t.Errorf("expected plugin.json path, got %q", path)
	}
}

func TestFindManifestFileNone(t *testing.T) {
	dir := t.TempDir()
	path := findManifestFile(dir)
	if path != "" {
		t.Errorf("expected empty path for dir with no manifest, got %q", path)
	}
}
