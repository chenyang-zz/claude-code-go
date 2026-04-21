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
