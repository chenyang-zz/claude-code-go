package autodream

import (
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

func TestIsAutoDreamEnabled_DisabledByDefault(t *testing.T) {
	// isAutoDreamEnabled should return false when CLAUDE_FEATURE_AUTO_DREAM is not set.
	if isAutoDreamEnabled() {
		t.Error("expected isAutoDreamEnabled to return false by default")
	}
}

func TestIsAutoDreamEnabled_EnabledViaFeatureFlag(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_AUTO_DREAM", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_AUTO_DREAM")

	if !featureflag.IsEnabled(featureflag.FlagAutoDream) {
		t.Error("expected FlagAutoDream to be enabled when env is set to 1")
	}
	if !isAutoDreamEnabled() {
		t.Error("expected isAutoDreamEnabled to return true when feature flag is set")
	}
}

func TestGetConfig_Defaults(t *testing.T) {
	cfg := getConfig()
	if cfg.MinHours != 24 {
		t.Errorf("expected default MinHours=24, got %d", cfg.MinHours)
	}
	if cfg.MinSessions != 5 {
		t.Errorf("expected default MinSessions=5, got %d", cfg.MinSessions)
	}
}

func TestGetConfig_EnvOverride(t *testing.T) {
	os.Setenv("CLAUDE_CODE_AUTO_DREAM_MIN_HOURS", "12")
	os.Setenv("CLAUDE_CODE_AUTO_DREAM_MIN_SESSIONS", "3")
	defer os.Unsetenv("CLAUDE_CODE_AUTO_DREAM_MIN_HOURS")
	defer os.Unsetenv("CLAUDE_CODE_AUTO_DREAM_MIN_SESSIONS")

	cfg := getConfig()
	if cfg.MinHours != 12 {
		t.Errorf("expected MinHours=12 from env, got %d", cfg.MinHours)
	}
	if cfg.MinSessions != 3 {
		t.Errorf("expected MinSessions=3 from env, got %d", cfg.MinSessions)
	}
}

func TestGetConfig_InvalidEnvValuesIgnored(t *testing.T) {
	os.Setenv("CLAUDE_CODE_AUTO_DREAM_MIN_HOURS", "notanumber")
	defer os.Unsetenv("CLAUDE_CODE_AUTO_DREAM_MIN_HOURS")

	cfg := getConfig()
	if cfg.MinHours != 24 {
		t.Errorf("expected default MinHours=24 when env is invalid, got %d", cfg.MinHours)
	}
}
