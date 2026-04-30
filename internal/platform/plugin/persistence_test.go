package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_LoadEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := &InstalledPluginsStore{
		filePath: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	file, err := store.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file.Version != 2 {
		t.Errorf("expected version 2, got %d", file.Version)
	}
	if len(file.Plugins) != 0 {
		t.Errorf("expected empty plugins, got %d", len(file.Plugins))
	}
}

func TestStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store := &InstalledPluginsStore{
		filePath: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	file := &InstalledPluginsFile{
		Version: 2,
		Plugins: map[string][]PluginInstallEntry{
			"test-plugin": {
				{Scope: "user", InstallPath: "/tmp/test-plugin", Version: "1.0.0"},
			},
		},
	}

	if err := store.Save(file); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if len(loaded.Plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(loaded.Plugins))
	}
	entries := loaded.Plugins["test-plugin"]
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Scope != "user" {
		t.Errorf("expected scope 'user', got '%s'", entries[0].Scope)
	}
	if entries[0].Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", entries[0].Version)
	}
}

func TestStore_AddPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	store := &InstalledPluginsStore{
		filePath: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	entry := PluginInstallEntry{
		Scope:       "user",
		InstallPath: "/tmp/test-plugin",
		Version:     "abc123",
	}
	if err := store.AddPlugin("test-plugin", entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := store.GetPlugin("test-plugin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Scope != "user" {
		t.Errorf("expected scope 'user', got '%s'", entries[0].Scope)
	}
}

func TestStore_AddPluginUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	store := &InstalledPluginsStore{
		filePath: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	entry1 := PluginInstallEntry{
		Scope:       "user",
		InstallPath: "/tmp/v1",
		Version:     "1.0.0",
	}
	if err := store.AddPlugin("test-plugin", entry1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry2 := PluginInstallEntry{
		Scope:       "user",
		InstallPath: "/tmp/v2",
		Version:     "2.0.0",
	}
	if err := store.AddPlugin("test-plugin", entry2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := store.GetPlugin("test-plugin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after update, got %d", len(entries))
	}
	if entries[0].Version != "2.0.0" {
		t.Errorf("expected version '2.0.0', got '%s'", entries[0].Version)
	}
}

func TestStore_AddPluginMultiScope(t *testing.T) {
	tmpDir := t.TempDir()
	store := &InstalledPluginsStore{
		filePath: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	if err := store.AddPlugin("test-plugin", PluginInstallEntry{
		Scope: "user", InstallPath: "/tmp/user", Version: "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.AddPlugin("test-plugin", PluginInstallEntry{
		Scope: "project", ProjectPath: "/myproject", InstallPath: "/tmp/proj", Version: "2.0.0",
	}); err != nil {
		t.Fatal(err)
	}

	entries, err := store.GetPlugin("test-plugin")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for multi-scope, got %d", len(entries))
	}
}

func TestStore_RemovePlugin(t *testing.T) {
	tmpDir := t.TempDir()
	store := &InstalledPluginsStore{
		filePath: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	if err := store.AddPlugin("test-plugin", PluginInstallEntry{
		Scope: "user", InstallPath: "/tmp/test-plugin", Version: "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.RemovePlugin("test-plugin", "user", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := store.GetPlugin("test-plugin")
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Errorf("expected nil entries after removal, got %d", len(entries))
	}
}

func TestStore_RemovePluginNoOp(t *testing.T) {
	tmpDir := t.TempDir()
	store := &InstalledPluginsStore{
		filePath: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	// Removing a non-existent plugin should be a no-op.
	if err := store.RemovePlugin("nonexistent", "user", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStore_GetPluginNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := &InstalledPluginsStore{
		filePath: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	entries, err := store.GetPlugin("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Errorf("expected nil for non-existent plugin")
	}
}

func TestStore_ListAll(t *testing.T) {
	tmpDir := t.TempDir()
	store := &InstalledPluginsStore{
		filePath: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	if err := store.AddPlugin("plugin-a", PluginInstallEntry{
		Scope: "user", InstallPath: "/tmp/a", Version: "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.AddPlugin("plugin-b", PluginInstallEntry{
		Scope: "user", InstallPath: "/tmp/b", Version: "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}

	all, err := store.ListAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(all))
	}
}

func TestStore_AutoTimestamps(t *testing.T) {
	tmpDir := t.TempDir()
	store := &InstalledPluginsStore{
		filePath: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	entry := PluginInstallEntry{
		Scope:       "user",
		InstallPath: "/tmp/test-plugin",
		Version:     "1.0.0",
		// InstalledAt is left empty — should be auto-filled.
	}
	if err := store.AddPlugin("test-plugin", entry); err != nil {
		t.Fatal(err)
	}

	entries, err := store.GetPlugin("test-plugin")
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].InstalledAt == "" {
		t.Error("expected InstalledAt to be auto-filled")
	}
	if entries[0].LastUpdated == "" {
		t.Error("expected LastUpdated to be auto-filled")
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "installed_plugins.json")

	done := make(chan bool, 10)
	for _ = range 10 {
		go func() {
			store := &InstalledPluginsStore{filePath: filePath}
			// Load without AddPlugin to avoid deadlock on separate store instances.
			_, _ = store.Load()
			done <- true
		}()
	}
	for _ = range 10 {
		<-done
	}
}

func TestStore_VersionDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	store := &InstalledPluginsStore{
		filePath: filepath.Join(tmpDir, "installed_plugins.json"),
	}

	// Write a file without a version field.
	if err := os.WriteFile(store.filePath, []byte(`{"plugins":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if file.Version != 2 {
		t.Errorf("expected version to default to 2, got %d", file.Version)
	}
}

func TestNewInstalledPluginsStore(t *testing.T) {
	store := NewInstalledPluginsStore()
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.filePath == "" {
		t.Error("expected non-empty file path")
	}
}
