package sessionmemorycompact

import "sync"

// Default config values for session memory compaction. These match the TS
// DEFAULT_SM_COMPACT_CONFIG values (minTokens=10000, minTextBlockMessages=5,
// maxTokens=40000).
var (
	// defaultSMCompactConfig holds the initial default configuration.
	defaultSMCompactConfig = SessionMemoryCompactConfig{
		MinTokens:            10_000,
		MinTextBlockMessages: 5,
		MaxTokens:            40_000,
	}
	// currentSMCompactConfig holds the current (possibly overridden) config.
	currentSMCompactConfig = defaultSMCompactConfig
	// configMu guards concurrent access to currentSMCompactConfig.
	configMu sync.RWMutex
)

// DefaultSMCompactConfig returns a copy of the default session memory compact
// configuration.
func DefaultSMCompactConfig() SessionMemoryCompactConfig {
	return defaultSMCompactConfig
}

// GetSessionMemoryCompactConfig returns a copy of the current session memory
// compact configuration.
func GetSessionMemoryCompactConfig() SessionMemoryCompactConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	return currentSMCompactConfig
}

// SetSessionMemoryCompactConfig updates the current configuration with any
// non-zero fields in the provided config. Zero-valued fields keep their
// current values.
func SetSessionMemoryCompactConfig(cfg SessionMemoryCompactConfig) {
	configMu.Lock()
	defer configMu.Unlock()
	if cfg.MinTokens > 0 {
		currentSMCompactConfig.MinTokens = cfg.MinTokens
	}
	if cfg.MinTextBlockMessages > 0 {
		currentSMCompactConfig.MinTextBlockMessages = cfg.MinTextBlockMessages
	}
	if cfg.MaxTokens > 0 {
		currentSMCompactConfig.MaxTokens = cfg.MaxTokens
	}
}

// ResetSessionMemoryCompactConfig resets the current configuration to defaults.
func ResetSessionMemoryCompactConfig() {
	configMu.Lock()
	defer configMu.Unlock()
	currentSMCompactConfig = defaultSMCompactConfig
}
