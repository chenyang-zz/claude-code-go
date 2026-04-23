package config

import (
	"testing"
)

// TestDefaultConfigEnablesPromptCachingByDefault verifies that prompt caching
// is enabled by default in the zero-value/default configuration.
func TestDefaultConfigEnablesPromptCachingByDefault(t *testing.T) {
	t.Setenv("DISABLE_PROMPT_CACHING", "")
	cfg := DefaultConfig()
	if !cfg.EnablePromptCaching {
		t.Fatalf("EnablePromptCaching = %v, want true", cfg.EnablePromptCaching)
	}
}

// TestDefaultConfigDisablesPromptCachingViaEnv verifies that setting the
// DISABLE_PROMPT_CACHING environment variable disables prompt caching.
func TestDefaultConfigDisablesPromptCachingViaEnv(t *testing.T) {
	t.Setenv("DISABLE_PROMPT_CACHING", "1")
	cfg := DefaultConfig()
	if cfg.EnablePromptCaching {
		t.Fatalf("EnablePromptCaching = %v, want false", cfg.EnablePromptCaching)
	}
}
