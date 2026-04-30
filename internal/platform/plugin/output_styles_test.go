package plugin

import (
	"path/filepath"
	"testing"
)

func TestExtractOutputStyles_EmptyPath(t *testing.T) {
	plugin := &LoadedPlugin{Name: "test", Path: "/tmp/test"}
	styles, err := ExtractOutputStyles(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if styles != nil {
		t.Errorf("expected nil, got %v", styles)
	}
}

func TestExtractOutputStyles_WithStylesDir(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	stylesPath := filepath.Join(pluginPath, "output-styles")
	mustMkdirAll(t, stylesPath)
	writeFile(t, filepath.Join(stylesPath, "compact.md"), `---
name: compact
description: Compact output style
force-for-plugin: true
---
Use compact formatting. Keep responses short.`)

	plugin := &LoadedPlugin{
		Name:             "test-plugin",
		Path:             pluginPath,
		OutputStylesPath: stylesPath,
	}

	styles, err := ExtractOutputStyles(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(styles))
	}
	if styles[0].Name != "test-plugin:compact" {
		t.Errorf("expected 'test-plugin:compact', got %q", styles[0].Name)
	}
	if styles[0].Description != "Compact output style" {
		t.Errorf("expected description, got %q", styles[0].Description)
	}
	if !styles[0].ForceForPlugin {
		t.Error("expected ForceForPlugin to be true")
	}
	if styles[0].Prompt != "Use compact formatting. Keep responses short." {
		t.Errorf("expected prompt, got %q", styles[0].Prompt)
	}
}

func TestExtractOutputStyles_NameFallback(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	stylesPath := filepath.Join(pluginPath, "output-styles")
	mustMkdirAll(t, stylesPath)
	// No "name" in frontmatter — should fall back to filename.
	writeFile(t, filepath.Join(stylesPath, "custom.md"), `# Custom Style
Style content.`)

	plugin := &LoadedPlugin{
		Name:             "test-plugin",
		Path:             pluginPath,
		OutputStylesPath: stylesPath,
	}

	styles, err := ExtractOutputStyles(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(styles))
	}
	if styles[0].Name != "test-plugin:custom" {
		t.Errorf("expected 'test-plugin:custom', got %q", styles[0].Name)
	}
}

func TestExtractOutputStyles_DescriptionFallback(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	stylesPath := filepath.Join(pluginPath, "output-styles")
	mustMkdirAll(t, stylesPath)
	// No description in frontmatter — should fall back to first paragraph.
	writeFile(t, filepath.Join(stylesPath, "no-desc.md"), `---
name: no-desc
---
# Heading

Fallback description paragraph.`)

	plugin := &LoadedPlugin{
		Name:             "test-plugin",
		Path:             pluginPath,
		OutputStylesPath: stylesPath,
	}

	styles, err := ExtractOutputStyles(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(styles))
	}
	if styles[0].Description != "Fallback description paragraph." {
		t.Errorf("expected fallback description, got %q", styles[0].Description)
	}
}

func TestExtractOutputStyles_MultipleStyles(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	stylesPath := filepath.Join(pluginPath, "output-styles")
	mustMkdirAll(t, stylesPath)
	writeFile(t, filepath.Join(stylesPath, "style-a.md"), `---
name: Style A
---
Content A.`)
	writeFile(t, filepath.Join(stylesPath, "style-b.md"), `---
name: Style B
force-for-plugin: false
---
Content B.`)

	plugin := &LoadedPlugin{
		Name:             "test-plugin",
		Path:             pluginPath,
		OutputStylesPath: stylesPath,
	}

	styles, err := ExtractOutputStyles(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(styles) != 2 {
		t.Fatalf("expected 2 styles, got %d", len(styles))
	}
}

func TestExtractOutputStyles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	stylesPath := filepath.Join(pluginPath, "output-styles")
	mustMkdirAll(t, stylesPath)

	plugin := &LoadedPlugin{
		Name:             "test-plugin",
		Path:             pluginPath,
		OutputStylesPath: stylesPath,
	}

	styles, err := ExtractOutputStyles(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(styles) != 0 {
		t.Errorf("expected 0 styles, got %d", len(styles))
	}
}
