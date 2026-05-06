package growthbook

import (
	"encoding/json"
	"os"
	"strings"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

var (
	// envOverrides lazily parsed from CLAUDE_INTERNAL_FC_OVERRIDES.
	envOverrides      map[string]interface{}
	envOverridesMu    sync.Mutex
	envOverridesReady bool
)

// isAnt returns true when the USER_TYPE environment variable equals "ant".
func isAnt() bool {
	return os.Getenv("USER_TYPE") == "ant"
}

// getEnvOverrides parses and returns env var overrides (ant-only).
func getEnvOverrides() map[string]interface{} {
	envOverridesMu.Lock()
	defer envOverridesMu.Unlock()

	if envOverridesReady {
		return envOverrides
	}
	envOverridesReady = true

	if !isAnt() {
		return nil
	}

	raw := os.Getenv("CLAUDE_INTERNAL_FC_OVERRIDES")
	if raw == "" {
		return nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		logger.WarnCF("growthbook", "failed to parse CLAUDE_INTERNAL_FC_OVERRIDES", map[string]interface{}{
			"error": err.Error(),
		})
		return nil
	}

	logger.DebugCF("growthbook", "using env var overrides for features", map[string]interface{}{
		"count": len(parsed),
	})
	envOverrides = parsed
	return envOverrides
}

// HasEnvOverride reports whether a feature has an env var override.
func HasEnvOverride(feature string) bool {
	overrides := getEnvOverrides()
	if overrides == nil {
		return false
	}
	_, ok := overrides[feature]
	return ok
}

// configOverrideProvider is the interface for reading/writing config overrides.
// This is set during initialization to avoid coupling to a specific config system.
type configOverrideProvider interface {
	GetOverrides() map[string]interface{}
	SetOverride(feature string, value interface{})
	ClearOverride(feature string)
	ClearAll()
	GetAllFeatures() map[string]interface{}
}

// defaultConfigProvider is a simple in-memory config override provider.
type defaultConfigProvider struct {
	mu        sync.RWMutex
	overrides map[string]interface{}
}

func (p *defaultConfigProvider) GetOverrides() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.overrides) == 0 {
		return nil
	}
	result := make(map[string]interface{}, len(p.overrides))
	for k, v := range p.overrides {
		result[k] = v
	}
	return result
}

func (p *defaultConfigProvider) SetOverride(feature string, value interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.overrides == nil {
		p.overrides = make(map[string]interface{})
	}
	p.overrides[feature] = value
}

func (p *defaultConfigProvider) ClearOverride(feature string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.overrides, feature)
}

func (p *defaultConfigProvider) ClearAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.overrides = nil
}

func (p *defaultConfigProvider) GetAllFeatures() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.overrides
}

var configOverrideProv configOverrideProvider

// SetConfigOverrideProvider sets the config override provider used by the package.
func SetConfigOverrideProvider(provider configOverrideProvider) {
	configOverrideProv = provider
}

func getConfigOverrides() map[string]interface{} {
	if configOverrideProv == nil {
		return nil
	}
	return configOverrideProv.GetOverrides()
}

// CheckEnvOverride checks env var overrides for the given feature.
// Returns the value and true if an override exists.
func CheckEnvOverride(feature string) (interface{}, bool) {
	overrides := getEnvOverrides()
	if overrides == nil {
		return nil, false
	}
	v, ok := overrides[feature]
	return v, ok
}

// CheckConfigOverride checks config overrides for the given feature.
// Returns the value and true if an override exists.
func CheckConfigOverride(feature string) (interface{}, bool) {
	overrides := getConfigOverrides()
	if overrides == nil {
		return nil, false
	}
	v, ok := overrides[feature]
	return v, ok
}

// checkOverrides checks both env and config overrides in priority order.
// Returns the value, source name, and whether an override was found.
func checkOverrides(feature string) (interface{}, string, bool) {
	if v, ok := CheckEnvOverride(feature); ok {
		return v, "envOverride", true
	}
	if v, ok := CheckConfigOverride(feature); ok {
		return v, "configOverride", true
	}
	return nil, "", false
}

// APIBaseURLHost returns the hostname of ANTHROPIC_BASE_URL when it points at
// a non-Anthropic proxy, or empty string otherwise.
func APIBaseURLHost() string {
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		return ""
	}
	// Strip protocol prefix for simple host extraction
	baseURL = strings.TrimPrefix(baseURL, "https://")
	baseURL = strings.TrimPrefix(baseURL, "http://")
	if idx := strings.Index(baseURL, "/"); idx > 0 {
		baseURL = baseURL[:idx]
	}
	if idx := strings.Index(baseURL, ":"); idx > 0 {
		baseURL = baseURL[:idx]
	}
	if baseURL == "api.anthropic.com" || baseURL == "" {
		return ""
	}
	return baseURL
}
