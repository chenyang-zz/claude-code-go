package featureflag

import (
	"os"
	"testing"
)

func TestIsEnabled_UnsetReturnsFalse(t *testing.T) {
	os.Unsetenv("CLAUDE_FEATURE_TOKEN_BUDGET")
	if IsEnabled(FlagTokenBudget) {
		t.Error("expected IsEnabled to return false when env is unset")
	}
}

func TestIsEnabled_ValueOneReturnsTrue(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_TOKEN_BUDGET", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_TOKEN_BUDGET")
	if !IsEnabled(FlagTokenBudget) {
		t.Error("expected IsEnabled to return true when env is '1'")
	}
}

func TestIsEnabled_OtherValuesReturnFalse(t *testing.T) {
	for _, val := range []string{"0", "true", "yes", "on", "TRUE", "Yes"} {
		os.Setenv("CLAUDE_FEATURE_TOKEN_BUDGET", val)
		if IsEnabled(FlagTokenBudget) {
			t.Errorf("expected IsEnabled to return false for env value %q", val)
		}
	}
	os.Unsetenv("CLAUDE_FEATURE_TOKEN_BUDGET")
}

func TestIsEnabled_UnknownFlagReturnsFalse(t *testing.T) {
	if IsEnabled("NONEXISTENT_FLAG") {
		t.Error("expected IsEnabled to return false for unknown flag")
	}
}

func TestIsEnabled_CustomFlag(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_CUSTOM", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_CUSTOM")
	if !IsEnabled("CUSTOM") {
		t.Error("expected IsEnabled to return true for custom flag set to '1'")
	}
}

func TestIsTodoV2Enabled_DefaultTrue(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ENABLE_TASKS")
	os.Unsetenv("CLAUDE_FEATURE_TODO_V2")
	if !IsTodoV2Enabled() {
		t.Error("expected IsTodoV2Enabled to return true by default")
	}
}

func TestIsTodoV2Enabled_EnableTasksEnv(t *testing.T) {
	for _, val := range []string{"1", "true", "yes", "TRUE", "Yes"} {
		os.Setenv("CLAUDE_CODE_ENABLE_TASKS", val)
		if !IsTodoV2Enabled() {
			t.Errorf("expected IsTodoV2Enabled to return true for CLAUDE_CODE_ENABLE_TASKS=%q", val)
		}
	}
	os.Unsetenv("CLAUDE_CODE_ENABLE_TASKS")
}

func TestIsTodoV2Enabled_FeatureFlagEnv(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_TODO_V2", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_TODO_V2")
	if !IsTodoV2Enabled() {
		t.Error("expected IsTodoV2Enabled to return true when CLAUDE_FEATURE_TODO_V2=1")
	}
}

func TestIsTodoV2Enabled_DisabledByZero(t *testing.T) {
	os.Setenv("CLAUDE_CODE_ENABLE_TASKS", "0")
	os.Setenv("CLAUDE_FEATURE_TODO_V2", "0")
	// Default is true, so even with zeros it returns true unless we change default.
	// This test documents the current behavior: zeros do not override the default.
	if !IsTodoV2Enabled() {
		t.Error("expected IsTodoV2Enabled to return true because zeros do not disable the default")
	}
	os.Unsetenv("CLAUDE_CODE_ENABLE_TASKS")
	os.Unsetenv("CLAUDE_FEATURE_TODO_V2")
}
