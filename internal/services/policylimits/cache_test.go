package policylimits

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCache_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	SetCacheHomeDir(tmpDir)
	defer SetCacheHomeDir("")

	restrictions := map[string]Restriction{
		"allow_remote_sessions":  {Allowed: false},
		"allow_product_feedback": {Allowed: true},
	}

	if err := SaveCache(restrictions); err != nil {
		t.Fatalf("SaveCache error: %v", err)
	}

	loaded, err := LoadCache()
	if err != nil {
		t.Fatalf("LoadCache error: %v", err)
	}

	if len(loaded) != len(restrictions) {
		t.Fatalf("expected %d restrictions, got %d", len(restrictions), len(loaded))
	}

	if loaded["allow_remote_sessions"].Allowed {
		t.Error("allow_remote_sessions should be false")
	}
	if !loaded["allow_product_feedback"].Allowed {
		t.Error("allow_product_feedback should be true")
	}
}

func TestCache_Load_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	SetCacheHomeDir(tmpDir)
	defer SetCacheHomeDir("")
	_ = ClearCache() // ensure sessionCache is clean

	loaded, err := LoadCache()
	if err != nil {
		t.Fatalf("LoadCache should not error on missing file: %v", err)
	}
	if loaded != nil {
		t.Error("missing file should return nil")
	}
}

func TestCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	SetCacheHomeDir(tmpDir)
	defer SetCacheHomeDir("")

	_ = SaveCache(map[string]Restriction{"allow_remote_sessions": {Allowed: false}})
	if err := ClearCache(); err != nil {
		t.Fatalf("ClearCache error: %v", err)
	}

	path := filepath.Join(tmpDir, ".claude", cacheFilename)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("cache file should be removed after ClearCache")
	}
}

func TestCache_SessionCache(t *testing.T) {
	tmpDir := t.TempDir()
	SetCacheHomeDir(tmpDir)
	defer SetCacheHomeDir("")

	restrictions := map[string]Restriction{
		"allow_remote_sessions": {Allowed: true},
	}
	_ = SaveCache(restrictions)

	// First load populates session cache
	_, _ = LoadCache()

	// Remove file to prove second load uses session cache
	path := filepath.Join(tmpDir, ".claude", cacheFilename)
	_ = os.Remove(path)

	loaded, err := LoadCache()
	if err != nil {
		t.Fatalf("LoadCache error: %v", err)
	}
	if loaded == nil {
		t.Error("session cache should survive file removal")
	}
}

func TestComputeChecksum(t *testing.T) {
	a := computeChecksum(map[string]Restriction{"x": {Allowed: true}})
	b := computeChecksum(map[string]Restriction{"x": {Allowed: true}})
	if a != b {
		t.Error("same restrictions should produce same checksum")
	}

	c := computeChecksum(map[string]Restriction{"x": {Allowed: false}})
	if a == c {
		t.Error("different restrictions should produce different checksum")
	}
}
