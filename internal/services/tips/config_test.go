package tips

import (
	"os"
	"testing"
)

func TestIsSpinnerTipsEnabled_Default(t *testing.T) {
	// Clean environment
	os.Unsetenv("CLAUDE_CODE_ENABLE_SPINNER_TIPS")
	os.Unsetenv("CLAUDE_FEATURE_SPINNER_TIPS")

	// Default should be true for REPL/CLI-first runtime
	if !IsSpinnerTipsEnabled() {
		t.Error("expected spinner tips to be enabled by default")
	}
}

func TestIsSpinnerTipsEnabled_EnvTruthy(t *testing.T) {
	os.Setenv("CLAUDE_CODE_ENABLE_SPINNER_TIPS", "1")
	defer os.Unsetenv("CLAUDE_CODE_ENABLE_SPINNER_TIPS")

	if !IsSpinnerTipsEnabled() {
		t.Error("expected enabled when env is truthy")
	}
}

func TestIsSpinnerTipsEnabled_EnvFalsy(t *testing.T) {
	os.Setenv("CLAUDE_CODE_ENABLE_SPINNER_TIPS", "0")
	defer os.Unsetenv("CLAUDE_CODE_ENABLE_SPINNER_TIPS")

	if IsSpinnerTipsEnabled() {
		t.Error("expected disabled when env is falsy")
	}
}

func TestIsSpinnerTipsEnabled_FeatureFlag(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ENABLE_SPINNER_TIPS")
	os.Setenv("CLAUDE_FEATURE_SPINNER_TIPS", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_SPINNER_TIPS")

	if !IsSpinnerTipsEnabled() {
		t.Error("expected enabled when feature flag is set")
	}
}

func TestIsSpinnerTipsEnabled_FeatureFlagDisabled(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ENABLE_SPINNER_TIPS")
	os.Setenv("CLAUDE_FEATURE_SPINNER_TIPS", "0")
	defer os.Unsetenv("CLAUDE_FEATURE_SPINNER_TIPS")

	// When feature flag is explicitly 0, default takes over (true)
	// This is consistent with IsEnabled behavior: only "1" means enabled
	if !IsSpinnerTipsEnabled() {
		t.Error("expected enabled by default when feature flag is not '1'")
	}
}

func TestIsEnvTruthy(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"1", true}, {"true", true}, {"yes", true}, {"on", true},
		{"0", false}, {"false", false}, {"no", false}, {"off", false},
		{"maybe", false}, {"", false},
	}
	for _, c := range cases {
		got := isEnvTruthy(c.input)
		if got != c.want {
			t.Errorf("isEnvTruthy(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestIsEnvFalsy(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"0", true}, {"false", true}, {"no", true}, {"off", true},
		{"1", false}, {"true", false}, {"yes", false}, {"on", false},
		{"maybe", false}, {"", false},
	}
	for _, c := range cases {
		got := isEnvFalsy(c.input)
		if got != c.want {
			t.Errorf("isEnvFalsy(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}
