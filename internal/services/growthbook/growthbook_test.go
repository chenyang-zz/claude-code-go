package growthbook

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUserAttributes(t *testing.T) {
	attrs := UserAttributes{
		ID:       "test-id",
		DeviceID: "test-device",
		Platform: "darwin",
	}
	if attrs.ID != "test-id" {
		t.Errorf("expected test-id, got %s", attrs.ID)
	}
	if attrs.Platform != "darwin" {
		t.Errorf("expected darwin, got %s", attrs.Platform)
	}
}

func TestStoredExperimentData(t *testing.T) {
	data := StoredExperimentData{
		ExperimentID: "exp-1",
		VariationID:  1,
	}
	if data.ExperimentID != "exp-1" {
		t.Errorf("expected exp-1, got %s", data.ExperimentID)
	}
	if data.VariationID != 1 {
		t.Errorf("expected 1, got %d", data.VariationID)
	}
}

func TestGetEnvOverrides(t *testing.T) {
	// Save and restore env
	oldUserType := os.Getenv("USER_TYPE")
	oldOverrides := os.Getenv("CLAUDE_INTERNAL_FC_OVERRIDES")
	defer func() {
		os.Setenv("USER_TYPE", oldUserType)
		os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", oldOverrides)
		resetEnvOverrides()
	}()

	os.Setenv("USER_TYPE", "ant")
	os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", `{"my_feature": true, "my_config": {"key": "val"}}`)
	resetEnvOverrides()

	overrides := getEnvOverrides()
	if overrides == nil {
		t.Fatal("expected non-nil overrides")
	}
	if v, ok := overrides["my_feature"]; !ok || v != true {
		t.Errorf("expected my_feature=true, got %v", v)
	}
}

func TestGetEnvOverridesNonAnt(t *testing.T) {
	oldUserType := os.Getenv("USER_TYPE")
	oldOverrides := os.Getenv("CLAUDE_INTERNAL_FC_OVERRIDES")
	defer func() {
		os.Setenv("USER_TYPE", oldUserType)
		os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", oldOverrides)
		resetEnvOverrides()
	}()

	os.Setenv("USER_TYPE", "external")
	os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", `{"my_feature": true}`)
	resetEnvOverrides()

	overrides := getEnvOverrides()
	if overrides != nil {
		t.Fatal("expected nil overrides for non-ant users")
	}
}

func TestHasEnvOverride(t *testing.T) {
	oldUserType := os.Getenv("USER_TYPE")
	oldOverrides := os.Getenv("CLAUDE_INTERNAL_FC_OVERRIDES")
	defer func() {
		os.Setenv("USER_TYPE", oldUserType)
		os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", oldOverrides)
		resetEnvOverrides()
	}()

	os.Setenv("USER_TYPE", "ant")
	os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", `{"test_flag": 42}`)
	resetEnvOverrides()

	if !HasEnvOverride("test_flag") {
		t.Error("expected HasEnvOverride to be true")
	}
	if HasEnvOverride("nonexistent") {
		t.Error("expected HasEnvOverride to be false for nonexistent flag")
	}
}

func TestCheckEnvOverride(t *testing.T) {
	oldUserType := os.Getenv("USER_TYPE")
	oldOverrides := os.Getenv("CLAUDE_INTERNAL_FC_OVERRIDES")
	defer func() {
		os.Setenv("USER_TYPE", oldUserType)
		os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", oldOverrides)
		resetEnvOverrides()
	}()

	os.Setenv("USER_TYPE", "ant")
	os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", `{"test_flag": 42}`)
	resetEnvOverrides()

	v, ok := CheckEnvOverride("test_flag")
	if !ok {
		t.Fatal("expected ok")
	}
	if v.(float64) != 42 {
		t.Errorf("expected 42, got %v", v)
	}

	_, ok = CheckEnvOverride("nonexistent")
	if ok {
		t.Error("expected not ok for nonexistent")
	}
}

func TestAPIBaseURLHost(t *testing.T) {
	tests := []struct {
		envValue string
		expected string
	}{
		{"", ""},
		{"https://api.anthropic.com", ""},
		{"https://proxy.example.com", "proxy.example.com"},
		{"http://custom.proxy:8080", "custom.proxy"},
	}
	for _, tt := range tests {
		t.Run(tt.envValue, func(t *testing.T) {
			old := os.Getenv("ANTHROPIC_BASE_URL")
			os.Setenv("ANTHROPIC_BASE_URL", tt.envValue)
			defer os.Setenv("ANTHROPIC_BASE_URL", old)

			got := APIBaseURLHost()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestCheckOverrides(t *testing.T) {
	oldUserType := os.Getenv("USER_TYPE")
	oldOverrides := os.Getenv("CLAUDE_INTERNAL_FC_OVERRIDES")
	defer func() {
		os.Setenv("USER_TYPE", oldUserType)
		os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", oldOverrides)
		resetEnvOverrides()
	}()

	os.Setenv("USER_TYPE", "ant")
	os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", `{"env_flag": "from_env"}`)
	resetEnvOverrides()

	// Set config override
	configOverrideProv = &defaultConfigProvider{}
	configOverrideProv.SetOverride("config_flag", "from_config")
	defer func() { configOverrideProv = nil }()

	// Env override takes precedence over config override
	v, source, ok := checkOverrides("env_flag")
	if !ok {
		t.Fatal("expected ok")
	}
	if source != "envOverride" {
		t.Errorf("expected envOverride source, got %s", source)
	}
	if v != "from_env" {
		t.Errorf("expected from_env, got %v", v)
	}

	// Config override works for items not in env
	v, source, ok = checkOverrides("config_flag")
	if !ok {
		t.Fatal("expected ok for config flag")
	}
	if source != "configOverride" {
		t.Errorf("expected configOverride source, got %s", source)
	}
	if v != "from_config" {
		t.Errorf("expected from_config, got %v", v)
	}

	// No override for missing key
	_, _, ok = checkOverrides("missing")
	if ok {
		t.Error("expected not ok for missing flag")
	}
}

func TestRemoteEvalCache(t *testing.T) {
	cache := newRemoteEvalCache()

	// Initially empty
	_, ok := cache.get("test")
	if ok {
		t.Error("expected not ok for empty cache")
	}

	// Set and get
	cache.set("test", "value")
	v, ok := cache.get("test")
	if !ok {
		t.Fatal("expected ok")
	}
	if v != "value" {
		t.Errorf("expected value, got %v", v)
	}

	// Snapshot
	snap := cache.snapshot()
	if len(snap) != 1 {
		t.Errorf("expected 1 item, got %d", len(snap))
	}

	// Clear
	cache.clear()
	_, ok = cache.get("test")
	if ok {
		t.Error("expected not ok after clear")
	}
}

func TestExperimentStore(t *testing.T) {
	store := newExperimentStore()

	// Initially empty
	_, ok := store.get("feature-x")
	if ok {
		t.Error("expected not ok for empty store")
	}

	// Set once
	data := StoredExperimentData{ExperimentID: "exp-1", VariationID: 1}
	store.set("feature-x", data)

	// Get
	got, ok := store.get("feature-x")
	if !ok {
		t.Fatal("expected ok")
	}
	if got.ExperimentID != "exp-1" {
		t.Errorf("expected exp-1, got %s", got.ExperimentID)
	}

	// SetNX - should not overwrite
	store.set("feature-x", StoredExperimentData{ExperimentID: "exp-2", VariationID: 2})
	got, _ = store.get("feature-x")
	if got.ExperimentID != "exp-1" {
		t.Errorf("expected exp-1 (not overwritten), got %s", got.ExperimentID)
	}

	// Clear
	store.clear()
	_, ok = store.get("feature-x")
	if ok {
		t.Error("expected not ok after clear")
	}
}

func TestSignal(t *testing.T) {
	s := &signal{}
	called := false

	unsub := s.subscribe(func() {
		called = true
	})

	s.emit()
	if !called {
		t.Error("expected listener to be called")
	}

	// Unsubscribe
	unsub()
	called = false
	s.emit()
	if called {
		t.Error("expected listener not to be called after unsubscribe")
	}
}

func TestSignalMultiple(t *testing.T) {
	s := &signal{}
	count := 0

	s.subscribe(func() { count++ })
	s.subscribe(func() { count++ })

	s.emit()
	if count != 2 {
		t.Errorf("expected 2 calls, got %d", count)
	}
}

func TestOnRefresh(t *testing.T) {
	called := false
	unsub := OnRefresh(func() {
		called = true
	})

	refreshed.emit()
	if !called {
		t.Error("expected OnRefresh listener to be called")
	}

	unsub()
}

func TestExposureLogging(t *testing.T) {
	// Set up experiment data
	data := StoredExperimentData{ExperimentID: "exp-1", VariationID: 1}
	experimentData.set("feature-a", data)

	// First call should log
	logExposureForFeature("feature-a")

	// Second call should be deduped (already logged)
	logExposureForFeature("feature-a")

	// Reset
	resetExposureTracking()
}

func TestPendingExposures(t *testing.T) {
	// Add pending exposures
	addPendingExposure("feature-a")
	addPendingExposure("feature-b")

	// Set up experiment data
	experimentData.set("feature-a", StoredExperimentData{ExperimentID: "exp-1", VariationID: 1})
	experimentData.set("feature-b", StoredExperimentData{ExperimentID: "exp-2", VariationID: 2})

	// Flush and verify no panic
	flushPendingExposures()

	resetExposureTracking()
}

func TestGetValueOverrideChain(t *testing.T) {
	// Set env overrides
	oldUserType := os.Getenv("USER_TYPE")
	oldOverrides := os.Getenv("CLAUDE_INTERNAL_FC_OVERRIDES")
	defer func() {
		os.Setenv("USER_TYPE", oldUserType)
		os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", oldOverrides)
		resetEnvOverrides()
	}()

	os.Setenv("USER_TYPE", "ant")
	os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", `{"env_flag": "env_value"}`)
	resetEnvOverrides()

	// Config override
	configOverrideProv = &defaultConfigProvider{}
	configOverrideProv.SetOverride("config_flag", "config_value")
	defer func() { configOverrideProv = nil }()

	// Env override
	v, source := GetValue("env_flag", "default")
	if v != "env_value" {
		t.Errorf("expected env_value, got %v", v)
	}
	if source != "envOverride" {
		t.Errorf("expected envOverride, got %s", source)
	}

	// Config override
	v, source = GetValue("config_flag", "default")
	if v != "config_value" {
		t.Errorf("expected config_value, got %v", v)
	}
	if source != "configOverride" {
		t.Errorf("expected configOverride, got %s", source)
	}

	// Default value
	v, source = GetValue("missing", "fallback")
	if v != "fallback" {
		t.Errorf("expected fallback, got %v", v)
	}
	if source != "defaultValue" {
		t.Errorf("expected defaultValue, got %s", source)
	}
}

func TestGetCached(t *testing.T) {
	// With env override
	oldUserType := os.Getenv("USER_TYPE")
	oldOverrides := os.Getenv("CLAUDE_INTERNAL_FC_OVERRIDES")
	defer func() {
		os.Setenv("USER_TYPE", oldUserType)
		os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", oldOverrides)
		resetEnvOverrides()
	}()

	os.Setenv("USER_TYPE", "ant")
	os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", `{"cached_flag": "cached_env"}`)
	resetEnvOverrides()

	// Env override should be returned
	v := GetCached("cached_flag", "default")
	if v != "cached_env" {
		t.Errorf("expected cached_env, got %v", v)
	}

	// Default for missing
	v = GetCached("missing", "fallback")
	if v != "fallback" {
		t.Errorf("expected fallback, got %v", v)
	}
}

func TestCheckGate(t *testing.T) {
	oldUserType := os.Getenv("USER_TYPE")
	oldOverrides := os.Getenv("CLAUDE_INTERNAL_FC_OVERRIDES")
	defer func() {
		os.Setenv("USER_TYPE", oldUserType)
		os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", oldOverrides)
		resetEnvOverrides()
	}()

	os.Setenv("USER_TYPE", "ant")
	os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", `{"gate_enabled": true, "gate_disabled": false}`)
	resetEnvOverrides()

	if !CheckGate("gate_enabled") {
		t.Error("expected gate_enabled to be true")
	}
	if CheckGate("gate_disabled") {
		t.Error("expected gate_disabled to be false")
	}
	if CheckGate("missing_gate") {
		t.Error("expected missing_gate to be false")
	}
}

func TestIsGrowthBookEnabled(t *testing.T) {
	// Default: enabled
	if !IsGrowthBookEnabled() {
		t.Error("expected enabled by default")
	}

	// Override
	SetEnabledFn(func() bool { return false })
	if IsGrowthBookEnabled() {
		t.Error("expected disabled after override")
	}

	// Reset
	SetEnabledFn(func() bool { return true })
}

func TestUserAttributesProvider(t *testing.T) {
	old := getUserAttributes
	defer func() { getUserAttributes = old }()

	SetUserAttributesProvider(func() UserAttributes {
		return UserAttributes{ID: "custom-id"}
	})

	attrs := getUserAttributes()
	if attrs.ID != "custom-id" {
		t.Errorf("expected custom-id, got %s", attrs.ID)
	}
}

func TestGrowthBookCachePath(t *testing.T) {
	oldCacheDir := os.Getenv("CLAUDE_CODE_CACHE_DIR")
	oldHome, _ := os.UserHomeDir()
	defer os.Setenv("CLAUDE_CODE_CACHE_DIR", oldCacheDir)

	// Custom cache dir
	os.Setenv("CLAUDE_CODE_CACHE_DIR", "/tmp/cache")
	path := growthBookCachePath()
	if path != "/tmp/cache/growthbook_features.json" {
		t.Errorf("expected /tmp/cache/growthbook_features.json, got %s", path)
	}

	// No custom dir, should use home
	os.Unsetenv("CLAUDE_CODE_CACHE_DIR")
	path = growthBookCachePath()
	expected := oldHome + "/.claude/growthbook_features.json"
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestDiskCacheRoundTrip(t *testing.T) {
	tmpFile := t.TempDir() + "/growthbook_features.json"
	oldCacheDir := os.Getenv("CLAUDE_CODE_CACHE_DIR")
	os.Setenv("CLAUDE_CODE_CACHE_DIR", t.TempDir())
	defer os.Setenv("CLAUDE_CODE_CACHE_DIR", oldCacheDir)

	// Write to disk cache via remoteEvalValues
	remoteEvalValues.set("test_feature", "test_value")
	remoteEvalValues.set("num_feature", float64(42))
	syncRemoteEvalToDisk()

	// Read back
	cache := loadDiskCache()
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if v, ok := cache["test_feature"]; !ok || v != "test_value" {
		t.Errorf("expected test_value, got %v", v)
	}
	if v, ok := cache["num_feature"]; !ok || v != float64(42) {
		t.Errorf("expected 42, got %v", v)
	}

	// Cleanup
	os.Remove(tmpFile)
	resetCaches()
}

func TestDiskCacheMissing(t *testing.T) {
	oldCacheDir := os.Getenv("CLAUDE_CODE_CACHE_DIR")
	os.Setenv("CLAUDE_CODE_CACHE_DIR", t.TempDir()+"/nonexistent")
	defer os.Setenv("CLAUDE_CODE_CACHE_DIR", oldCacheDir)

	cache := loadDiskCache()
	if cache != nil {
		t.Error("expected nil for missing cache file")
	}
}

func TestProcessRemoteEvalPayload(t *testing.T) {
	// Simulate a remote eval payload like the GrowthBook API would return
	payload := map[string]json.RawMessage{
		"feature_a": json.RawMessage(`{"defaultValue": "value_a", "source": "experiment", "experimentResult": {"variationId": 0}, "experiment": {"key": "exp_a"}}`),
		"feature_b": json.RawMessage(`{"defaultValue": 123}`),
	}

	processRemoteEvalPayload(payload)

	// Check cached values
	v, ok := remoteEvalValues.get("feature_a")
	if !ok {
		t.Fatal("expected feature_a to be in cache")
	}
	if v != "value_a" {
		t.Errorf("expected value_a, got %v", v)
	}

	v, ok = remoteEvalValues.get("feature_b")
	if !ok {
		t.Fatal("expected feature_b to be in cache")
	}
	if v != float64(123) {
		t.Errorf("expected 123, got %v", v)
	}

	// Check experiment data
	data, ok := experimentData.get("feature_a")
	if !ok {
		t.Fatal("expected feature_a experiment data")
	}
	if data.ExperimentID != "exp_a" {
		t.Errorf("expected exp_a, got %s", data.ExperimentID)
	}

	resetCaches()
}

func TestProcessRemoteEvalPayloadMalformed(t *testing.T) {
	// Simulate malformed API response where "value" is used instead of "defaultValue"
	payload := map[string]json.RawMessage{
		"malformed_feature": json.RawMessage(`{"value": "from_value_field"}`),
	}

	processRemoteEvalPayload(payload)

	v, ok := remoteEvalValues.get("malformed_feature")
	if !ok {
		t.Fatal("expected malformed_feature to be in cache")
	}
	if v != "from_value_field" {
		t.Errorf("expected from_value_field, got %v", v)
	}

	resetCaches()
}

func TestProcessRemoteEvalPayloadEmpty(t *testing.T) {
	// Empty payload should not panic
	processRemoteEvalPayload(nil)
	processRemoteEvalPayload(make(map[string]json.RawMessage))
}

func TestInitConfig(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		ClientKey: "test-key",
	}
	if !cfg.Enabled {
		t.Error("expected enabled")
	}
	if cfg.ClientKey != "test-key" {
		t.Errorf("expected test-key, got %s", cfg.ClientKey)
	}
}

func TestClientConfig(t *testing.T) {
	cfg := ClientConfig{
		APIHost:   "https://custom.api.com",
		ClientKey: "custom-key",
		Enabled:   true,
	}
	if cfg.APIHost != "https://custom.api.com" {
		t.Errorf("expected custom api host, got %s", cfg.APIHost)
	}
}

func TestConcurrentCacheAccess(t *testing.T) {
	cache := newRemoteEvalCache()
	done := make(chan bool)

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			cache.set("key", i)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			cache.get("key")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			cache.snapshot()
		}
		done <- true
	}()

	for i := 0; i < 3; i++ {
		<-done
	}
}

func TestConcurrentExposureTracking(t *testing.T) {
	done := make(chan bool)

	go func() {
		for i := 0; i < 50; i++ {
			logExposureForFeature("concurrent-feature")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			addPendingExposure("concurrent-feature")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			isExposureLogged("concurrent-feature")
		}
		done <- true
	}()

	for i := 0; i < 3; i++ {
		<-done
	}

	resetExposureTracking()
}

func TestResetCaches(t *testing.T) {
	remoteEvalValues.set("test", "value")
	experimentData.set("test", StoredExperimentData{ExperimentID: "exp"})

	resetCaches()

	_, ok := remoteEvalValues.get("test")
	if ok {
		t.Error("expected empty after reset")
	}
	_, ok = experimentData.get("test")
	if ok {
		t.Error("expected empty after reset")
	}
}

func TestDefaultConfigProvider(t *testing.T) {
	p := &defaultConfigProvider{}

	// Initially empty
	if overrides := p.GetOverrides(); overrides != nil {
		t.Error("expected nil initially")
	}

	// Set and get
	p.SetOverride("key1", "val1")
	p.SetOverride("key2", 42)

	overrides := p.GetOverrides()
	if len(overrides) != 2 {
		t.Errorf("expected 2 overrides, got %d", len(overrides))
	}

	// Clear one
	p.ClearOverride("key1")
	overrides = p.GetOverrides()
	if len(overrides) != 1 {
		t.Errorf("expected 1 override after clear, got %d", len(overrides))
	}

	// Clear all
	p.ClearAll()
	overrides = p.GetOverrides()
	if overrides != nil {
		t.Error("expected nil after ClearAll")
	}
}

func TestClientKeyFromEnv(t *testing.T) {
	oldKey := os.Getenv("CLAUDE_CODE_GROWTHBOOK_CLIENT_KEY")
	oldClientKey := os.Getenv("CLAUDE_CODE_CLIENT_KEY")
	defer func() {
		os.Setenv("CLAUDE_CODE_GROWTHBOOK_CLIENT_KEY", oldKey)
		os.Setenv("CLAUDE_CODE_CLIENT_KEY", oldClientKey)
	}()

	os.Setenv("CLAUDE_CODE_GROWTHBOOK_CLIENT_KEY", "gb-key")
	os.Setenv("CLAUDE_CODE_CLIENT_KEY", "client-key")

	key := ClientKeyFromEnv()
	if key != "gb-key" {
		t.Errorf("expected gb-key (precedence), got %s", key)
	}

	os.Unsetenv("CLAUDE_CODE_GROWTHBOOK_CLIENT_KEY")
	key = ClientKeyFromEnv()
	if key != "client-key" {
		t.Errorf("expected client-key (fallback), got %s", key)
	}
}

func BenchmarkGetCachedOverride(b *testing.B) {
	oldUserType := os.Getenv("USER_TYPE")
	oldOverrides := os.Getenv("CLAUDE_INTERNAL_FC_OVERRIDES")
	defer func() {
		os.Setenv("USER_TYPE", oldUserType)
		os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", oldOverrides)
		resetEnvOverrides()
	}()

	os.Setenv("USER_TYPE", "ant")
	os.Setenv("CLAUDE_INTERNAL_FC_OVERRIDES", `{"bench_flag": "bench_value"}`)
	resetEnvOverrides()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetCached("bench_flag", "default")
	}
}

// Helper to reset env override state between tests
func resetEnvOverrides() {
	envOverridesMu.Lock()
	defer envOverridesMu.Unlock()
	envOverrides = nil
	envOverridesReady = false
}
