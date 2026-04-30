package plugin

import (
	"sync"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.Count() != 0 {
		t.Errorf("expected empty registry, got count %d", r.Count())
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	p := &LoadedPlugin{
		Name:    "test-plugin",
		Enabled: true,
		Manifest: PluginManifest{
			Name:    "test-plugin",
			Version: "1.0.0",
		},
	}

	if err := r.Register(p); err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	if r.Count() != 1 {
		t.Errorf("expected count 1, got %d", r.Count())
	}
}

func TestRegistryRegisterNil(t *testing.T) {
	r := NewRegistry()
	err := r.Register(nil)
	if err == nil {
		t.Error("expected error for nil plugin")
	}
}

func TestRegistryRegisterEmptyName(t *testing.T) {
	r := NewRegistry()
	err := r.Register(&LoadedPlugin{Name: ""})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestRegistryRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	p1 := &LoadedPlugin{Name: "plugin", Manifest: PluginManifest{Name: "plugin"}}
	if err := r.Register(p1); err != nil {
		t.Fatalf("unexpected first register error: %v", err)
	}

	p2 := &LoadedPlugin{Name: "plugin", Manifest: PluginManifest{Name: "plugin", Version: "2.0.0"}}
	err := r.Register(p2)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
	// First plugin should still be the one in the registry.
	if got := r.Get("plugin"); got != p1 {
		t.Error("expected original plugin to remain after duplicate register attempt")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	p := &LoadedPlugin{Name: "plugin-a", Enabled: true, Manifest: PluginManifest{Name: "plugin-a"}}
	r.Register(p)

	got := r.Get("plugin-a")
	if got != p {
		t.Error("expected to retrieve registered plugin")
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()
	got := r.Get("nonexistent")
	if got != nil {
		t.Error("expected nil for unregistered plugin")
	}
}

func TestRegistryUnregister(t *testing.T) {
	r := NewRegistry()
	p := &LoadedPlugin{Name: "plugin-a", Manifest: PluginManifest{Name: "plugin-a"}}
	r.Register(p)

	r.Unregister("plugin-a")
	if r.Count() != 0 {
		t.Errorf("expected count 0 after unregister, got %d", r.Count())
	}
	if got := r.Get("plugin-a"); got != nil {
		t.Error("expected nil after unregister")
	}
}

func TestRegistryUnregisterNoOp(t *testing.T) {
	r := NewRegistry()
	p := &LoadedPlugin{Name: "plugin-a", Manifest: PluginManifest{Name: "plugin-a"}}
	r.Register(p)

	// Unregistering a non-existent plugin is a no-op.
	r.Unregister("nonexistent")
	if r.Count() != 1 {
		t.Errorf("expected count unchanged, got %d", r.Count())
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	r.Register(&LoadedPlugin{Name: "a", Manifest: PluginManifest{Name: "a"}})
	r.Register(&LoadedPlugin{Name: "b", Manifest: PluginManifest{Name: "b"}})
	r.Register(&LoadedPlugin{Name: "c", Manifest: PluginManifest{Name: "c"}})

	list := r.List()
	if len(list) != 3 {
		t.Errorf("expected 3 plugins in list, got %d", len(list))
	}

	names := make(map[string]bool)
	for _, p := range list {
		names[p.Name] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !names[want] {
			t.Errorf("expected %q in list", want)
		}
	}
}

func TestRegistryListEmpty(t *testing.T) {
	r := NewRegistry()
	list := r.List()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d entries", len(list))
	}
}

func TestRegistrySetEnabled(t *testing.T) {
	r := NewRegistry()
	p := &LoadedPlugin{Name: "plugin", Enabled: true, Manifest: PluginManifest{Name: "plugin"}}
	r.Register(p)

	r.SetEnabled("plugin", false)
	if p.Enabled {
		t.Error("expected plugin to be disabled")
	}

	r.SetEnabled("plugin", true)
	if !p.Enabled {
		t.Error("expected plugin to be re-enabled")
	}
}

func TestRegistrySetEnabledNoOp(t *testing.T) {
	r := NewRegistry()
	// Should not panic when plugin not found.
	r.SetEnabled("nonexistent", true)
}

func TestRegistryClear(t *testing.T) {
	r := NewRegistry()
	r.Register(&LoadedPlugin{Name: "a", Manifest: PluginManifest{Name: "a"}})
	r.Register(&LoadedPlugin{Name: "b", Manifest: PluginManifest{Name: "b"}})

	r.Clear()
	if r.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", r.Count())
	}
	if len(r.List()) != 0 {
		t.Error("expected empty list after clear")
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "plugin-" + string(rune('A'+idx%26)) + string(rune('0'+idx/26))
			r.Register(&LoadedPlugin{Name: name, Manifest: PluginManifest{Name: name}})
		}(i)
	}
	wg.Wait()

	// Concurrent reads and writes.
	wg.Add(50)
	for i := 0; i < 50; i++ {
		go func(idx int) {
			defer wg.Done()
			r.List()
			r.Get("plugin-A0")
			r.Count()
			r.SetEnabled("plugin-A0", idx%2 == 0)
		}(i)
	}
	wg.Wait()
}

func TestRegistryRegisterThenGet(t *testing.T) {
	r := NewRegistry()
	manifest := PluginManifest{
		Name:        "full-plugin",
		Version:     "3.0.0",
		Description: "Full featured plugin",
		Author:      &PluginAuthor{Name: "Dev", Email: "dev@example.com"},
		Homepage:    "https://plugin.example.com",
		License:     "Apache-2.0",
	}
	p := &LoadedPlugin{
		Name:      "full-plugin",
		Manifest:  manifest,
		Path:      "/path/to/full-plugin",
		Source:    PluginSource{Type: SourceTypeGitHub, Value: "org/full-plugin"},
		Enabled:   true,
		IsBuiltin: false,
	}
	r.Register(p)

	got := r.Get("full-plugin")
	if got == nil {
		t.Fatal("expected to get plugin back")
	}
	if got.Manifest.Version != "3.0.0" {
		t.Errorf("expected version %q, got %q", "3.0.0", got.Manifest.Version)
	}
	if got.Manifest.Author == nil || got.Manifest.Author.Email != "dev@example.com" {
		t.Error("author info not preserved")
	}
	if got.Source.Type != SourceTypeGitHub {
		t.Errorf("expected source type github, got %q", got.Source.Type)
	}
}
