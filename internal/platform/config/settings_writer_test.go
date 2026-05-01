package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSettingsWriter(t *testing.T) {
	w := NewSettingsWriter("/home/user", "/tmp/project")
	if w.userPath != "/home/user/.claude/settings.json" {
		t.Errorf("userPath = %q, want %q", w.userPath, "/home/user/.claude/settings.json")
	}
	if w.projectPath != "/tmp/project/.claude/settings.json" {
		t.Errorf("projectPath = %q, want %q", w.projectPath, "/tmp/project/.claude/settings.json")
	}
	if w.localPath != "/tmp/project/.claude/settings.local.json" {
		t.Errorf("localPath = %q, want %q", w.localPath, "/tmp/project/.claude/settings.local.json")
	}
}

func TestNewSettingsWriter_EmptyDirs(t *testing.T) {
	w := NewSettingsWriter("", "")
	if w.userPath != "" {
		t.Error("userPath should be empty")
	}
	if w.projectPath != "" {
		t.Error("projectPath should be empty")
	}
}

func TestSettingsWriter_ResolvePath(t *testing.T) {
	w := NewSettingsWriter("/home/user", "/tmp/project")

	tests := []struct {
		scope    string
		wantPath string
		wantErr  bool
	}{
		{"user", "/home/user/.claude/settings.json", false},
		{"project", "/tmp/project/.claude/settings.json", false},
		{"local", "/tmp/project/.claude/settings.local.json", false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			path, err := w.resolvePath(tt.scope)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
		})
	}
}

func TestSettingsWriter_ResolvePath_Unavailable(t *testing.T) {
	w := NewSettingsWriter("", "")
	_, err := w.resolvePath("user")
	if err == nil {
		t.Error("expected error for unavailable user scope")
	}
	_, err = w.resolvePath("project")
	if err == nil {
		t.Error("expected error for unavailable project scope")
	}
}

func TestSettingsWriter_Get(t *testing.T) {
	dir := t.TempDir()
	w := NewSettingsWriter(dir, dir)

	// Write a settings file with known content
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(settingsPath), 0o755)
	os.WriteFile(settingsPath, []byte(`{"model":"sonnet","theme":"dark","permissions":{"defaultMode":"plan"}}`), 0o644)

	ctx := context.Background()

	// Simple key
	val, err := w.Get(ctx, "user", "model")
	if err != nil {
		t.Fatalf("Get model: %v", err)
	}
	if val != "sonnet" {
		t.Errorf("model = %v, want sonnet", val)
	}

	// Nested key
	val, err = w.Get(ctx, "user", "permissions.defaultMode")
	if err != nil {
		t.Fatalf("Get permissions.defaultMode: %v", err)
	}
	if val != "plan" {
		t.Errorf("permissions.defaultMode = %v, want plan", val)
	}

	// Non-existent key
	val, err = w.Get(ctx, "user", "nonexistent")
	if err != nil {
		t.Fatalf("Get nonexistent: %v", err)
	}
	if val != nil {
		t.Errorf("nonexistent = %v, want nil", val)
	}
}

func TestSettingsWriter_Get_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	w := NewSettingsWriter(dir, dir)

	ctx := context.Background()
	val, err := w.Get(ctx, "user", "model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil for non-existent file, got %v", val)
	}
}

func TestSettingsWriter_Set_SimpleKey(t *testing.T) {
	dir := t.TempDir()
	w := NewSettingsWriter(dir, dir)

	ctx := context.Background()
	if err := w.Set(ctx, "user", "theme", "light"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, err := w.Get(ctx, "user", "theme")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "light" {
		t.Errorf("theme = %v, want light", val)
	}
}

func TestSettingsWriter_Set_NestedKey(t *testing.T) {
	dir := t.TempDir()
	w := NewSettingsWriter(dir, dir)

	ctx := context.Background()
	if err := w.Set(ctx, "user", "permissions.defaultMode", "plan"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, err := w.Get(ctx, "user", "permissions.defaultMode")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "plan" {
		t.Errorf("permissions.defaultMode = %v, want plan", val)
	}
}

func TestSettingsWriter_Set_Overwrite(t *testing.T) {
	dir := t.TempDir()
	w := NewSettingsWriter(dir, dir)

	ctx := context.Background()

	// Write initial value
	if err := w.Set(ctx, "user", "theme", "dark"); err != nil {
		t.Fatalf("Set initial: %v", err)
	}
	// Overwrite
	if err := w.Set(ctx, "user", "theme", "light"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}

	val, _ := w.Get(ctx, "user", "theme")
	if val != "light" {
		t.Errorf("theme = %v, want light", val)
	}
}

func TestSettingsWriter_Set_PreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	w := NewSettingsWriter(dir, dir)

	ctx := context.Background()

	// Write two fields
	if err := w.Set(ctx, "user", "theme", "dark"); err != nil {
		t.Fatalf("Set theme: %v", err)
	}
	if err := w.Set(ctx, "user", "model", "sonnet"); err != nil {
		t.Fatalf("Set model: %v", err)
	}

	// Verify both fields exist
	theme, _ := w.Get(ctx, "user", "theme")
	model, _ := w.Get(ctx, "user", "model")
	if theme != "dark" {
		t.Errorf("theme = %v, want dark", theme)
	}
	if model != "sonnet" {
		t.Errorf("model = %v, want sonnet", model)
	}
}

func TestSetNestedKey(t *testing.T) {
	tests := []struct {
		name  string
		doc   map[string]any
		path  []string
		value any
		want  map[string]any
	}{
		{
			name:  "simple key",
			doc:   map[string]any{},
			path:  []string{"theme"},
			value: "dark",
			want:  map[string]any{"theme": "dark"},
		},
		{
			name:  "nested key",
			doc:   map[string]any{},
			path:  []string{"permissions", "defaultMode"},
			value: "plan",
			want:  map[string]any{"permissions": map[string]any{"defaultMode": "plan"}},
		},
		{
			name:  "deep nested new",
			doc:   map[string]any{},
			path:  []string{"a", "b", "c"},
			value: "value",
			want:  map[string]any{"a": map[string]any{"b": map[string]any{"c": "value"}}},
		},
		{
			name: "overwrite nested",
			doc: map[string]any{
				"permissions": map[string]any{"defaultMode": "plan", "other": true},
			},
			path:  []string{"permissions", "defaultMode"},
			value: "auto",
			want: map[string]any{
				"permissions": map[string]any{"defaultMode": "auto", "other": true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setNestedKey(tt.doc, tt.path, tt.value)
			if !mapsEqual(tt.doc, tt.want) {
				t.Errorf("doc = %v, want %v", tt.doc, tt.want)
			}
		})
	}
}

func TestGetNestedValue(t *testing.T) {
	doc := map[string]any{
		"theme": "dark",
		"permissions": map[string]any{
			"defaultMode": "plan",
		},
	}

	tests := []struct {
		path    []string
		want    any
		wantOk  bool
	}{
		{[]string{"theme"}, "dark", true},
		{[]string{"permissions", "defaultMode"}, "plan", true},
		{[]string{"nonexistent"}, nil, false},
		{[]string{"permissions", "nonexistent"}, nil, false},
		{[]string{}, nil, false},
	}

	for _, tt := range tests {
		val, ok := getNestedValue(doc, tt.path)
		if ok != tt.wantOk {
			t.Errorf("getNestedValue(%v) ok = %v, want %v", tt.path, ok, tt.wantOk)
		}
		if val != tt.want {
			t.Errorf("getNestedValue(%v) = %v, want %v", tt.path, val, tt.want)
		}
	}
}

func TestBuildNestedMap(t *testing.T) {
	result := buildNestedMap([]string{"a", "b"}, "v")
	expected := map[string]any{"a": map[string]any{"b": "v"}}
	if !mapsEqual(result, expected) {
		t.Errorf("buildNestedMap = %v, want %v", result, expected)
	}
}

func TestSettingsWriter_Unset_SimpleKey(t *testing.T) {
	dir := t.TempDir()
	w := NewSettingsWriter(dir, dir)

	ctx := context.Background()
	if err := w.Set(ctx, "user", "theme", "dark"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := w.Unset(ctx, "user", "theme"); err != nil {
		t.Fatalf("Unset: %v", err)
	}

	val, err := w.Get(ctx, "user", "theme")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != nil {
		t.Errorf("theme = %v, want nil after Unset", val)
	}
}

func TestSettingsWriter_Unset_NestedKey(t *testing.T) {
	dir := t.TempDir()
	w := NewSettingsWriter(dir, dir)

	ctx := context.Background()
	if err := w.Set(ctx, "user", "permissions.defaultMode", "plan"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := w.Unset(ctx, "user", "permissions.defaultMode"); err != nil {
		t.Fatalf("Unset: %v", err)
	}

	val, err := w.Get(ctx, "user", "permissions.defaultMode")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != nil {
		t.Errorf("permissions.defaultMode = %v, want nil after Unset", val)
	}
}

func TestSettingsWriter_Unset_NonExistentKey(t *testing.T) {
	dir := t.TempDir()
	w := NewSettingsWriter(dir, dir)

	ctx := context.Background()
	if err := w.Unset(ctx, "user", "nonexistent"); err != nil {
		t.Fatalf("Unset non-existent key should succeed, got %v", err)
	}
}

func TestSettingsWriter_Unset_EmptyDocumentRemovesFile(t *testing.T) {
	dir := t.TempDir()
	w := NewSettingsWriter(dir, dir)

	ctx := context.Background()
	if err := w.Set(ctx, "user", "theme", "dark"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := w.Unset(ctx, "user", "theme"); err != nil {
		t.Fatalf("Unset: %v", err)
	}

	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Errorf("settings file should be removed when empty, stat err=%v", err)
	}
}

func TestSettingsWriter_Unset_PreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	w := NewSettingsWriter(dir, dir)

	ctx := context.Background()
	if err := w.Set(ctx, "user", "theme", "dark"); err != nil {
		t.Fatalf("Set theme: %v", err)
	}
	if err := w.Set(ctx, "user", "model", "sonnet"); err != nil {
		t.Fatalf("Set model: %v", err)
	}
	if err := w.Unset(ctx, "user", "theme"); err != nil {
		t.Fatalf("Unset theme: %v", err)
	}

	model, err := w.Get(ctx, "user", "model")
	if err != nil {
		t.Fatalf("Get model: %v", err)
	}
	if model != "sonnet" {
		t.Errorf("model = %v, want sonnet", model)
	}
}

func TestUnsetNestedKey(t *testing.T) {
	tests := []struct {
		name string
		doc  map[string]any
		path []string
		want map[string]any
	}{
		{
			name: "simple key",
			doc:  map[string]any{"theme": "dark"},
			path: []string{"theme"},
			want: map[string]any{},
		},
		{
			name: "nested key",
			doc:  map[string]any{"permissions": map[string]any{"defaultMode": "plan"}},
			path: []string{"permissions", "defaultMode"},
			want: map[string]any{},
		},
		{
			name: "nested key keeps siblings",
			doc:  map[string]any{"permissions": map[string]any{"defaultMode": "plan", "other": true}},
			path: []string{"permissions", "defaultMode"},
			want: map[string]any{"permissions": map[string]any{"other": true}},
		},
		{
			name: "non-existent key",
			doc:  map[string]any{"theme": "dark"},
			path: []string{"nonexistent"},
			want: map[string]any{"theme": "dark"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetNestedKey(tt.doc, tt.path)
			if !mapsEqual(tt.doc, tt.want) {
				t.Errorf("doc = %v, want %v", tt.doc, tt.want)
			}
		})
	}
}

func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		ma, aMap := va.(map[string]any)
		mb, bMap := vb.(map[string]any)
		if aMap && bMap {
			if !mapsEqual(ma, mb) {
				return false
			}
			continue
		}
		if va != vb {
			return false
		}
	}
	return true
}
