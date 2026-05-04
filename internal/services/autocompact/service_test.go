package autocompact

import (
	"os"
	"testing"
)

func TestGetEffectiveContextWindowSize_KnownModel(t *testing.T) {
	size := GetEffectiveContextWindowSize("claude-sonnet-4-20250514")
	// 200_000 - min(8192, 20000) = 200_000 - 8192 = 191808
	expected := 200_000 - min(8192, MAX_OUTPUT_TOKENS_FOR_SUMMARY)
	if size != expected {
		t.Errorf("expected %d, got %d", expected, size)
	}
}

func TestGetEffectiveContextWindowSize_UnknownModel(t *testing.T) {
	size := GetEffectiveContextWindowSize("unknown-model")
	expected := defaultContextWindow - min(8192, MAX_OUTPUT_TOKENS_FOR_SUMMARY)
	if size != expected {
		t.Errorf("expected %d, got %d", expected, size)
	}
}

func TestGetEffectiveContextWindowSize_EnvOverride(t *testing.T) {
	os.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "50000")
	defer os.Unsetenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW")

	size := GetEffectiveContextWindowSize("claude-sonnet-4-20250514")
	// 50000 - min(8192, 20000) = 50000 - 8192 = 41808
	expected := 50000 - min(8192, MAX_OUTPUT_TOKENS_FOR_SUMMARY)
	if size != expected {
		t.Errorf("expected %d, got %d", expected, size)
	}
}

func TestGetAutoCompactThreshold(t *testing.T) {
	threshold := GetAutoCompactThreshold("claude-sonnet-4-20250514")
	// 191808 - 13000 = 178808
	effectiveWindow := GetEffectiveContextWindowSize("claude-sonnet-4-20250514")
	expected := effectiveWindow - AUTOCOMPACT_BUFFER_TOKENS
	if threshold != expected {
		t.Errorf("expected %d, got %d", expected, threshold)
	}
}

func TestGetAutoCompactThreshold_PctOverride(t *testing.T) {
	os.Setenv("CLAUDE_AUTOCOMPACT_PCT_OVERRIDE", "50")
	defer os.Unsetenv("CLAUDE_AUTOCOMPACT_PCT_OVERRIDE")

	threshold := GetAutoCompactThreshold("claude-sonnet-4-20250514")
	effectiveWindow := GetEffectiveContextWindowSize("claude-sonnet-4-20250514")
	expected := int(float64(effectiveWindow) * 0.5)
	if threshold != expected {
		t.Errorf("pct override: expected %d, got %d", expected, threshold)
	}
}

func TestGetAutoCompactThreshold_PctOverrideGtDefault(t *testing.T) {
	os.Setenv("CLAUDE_AUTOCOMPACT_PCT_OVERRIDE", "99")
	defer os.Unsetenv("CLAUDE_AUTOCOMPACT_PCT_OVERRIDE")

	// With 99% override, the percentage-based threshold would be 99% of
	// effective window, which is larger than the default threshold
	// (effectiveWindow - 13000), so default threshold should win.
	threshold := GetAutoCompactThreshold("claude-sonnet-4-20250514")
	effectiveWindow := GetEffectiveContextWindowSize("claude-sonnet-4-20250514")
	defaultThreshold := effectiveWindow - AUTOCOMPACT_BUFFER_TOKENS
	if threshold != defaultThreshold {
		t.Errorf("with 99%% pct, expected default %d, got %d", defaultThreshold, threshold)
	}
}

func TestCalculateTokenWarningState_BelowThreshold(t *testing.T) {
	// Use very low token usage
	state := CalculateTokenWarningState(1000, "claude-sonnet-4-20250514")
	if state.IsAboveWarningThreshold {
		t.Error("expected IsAboveWarningThreshold=false for low usage")
	}
	if state.IsAboveErrorThreshold {
		t.Error("expected IsAboveErrorThreshold=false for low usage")
	}
	if state.IsAboveAutoCompactThreshold {
		t.Error("expected IsAboveAutoCompactThreshold=false for low usage")
	}
	if state.IsAtBlockingLimit {
		t.Error("expected IsAtBlockingLimit=false for low usage")
	}
}

func TestCalculateTokenWarningState_AboveThreshold(t *testing.T) {
	effectiveWindow := GetEffectiveContextWindowSize("claude-sonnet-4-20250514")
	// Use usage near effective window to trigger thresholds
	state := CalculateTokenWarningState(effectiveWindow-2000, "claude-sonnet-4-20250514")
	if !state.IsAboveWarningThreshold {
		t.Error("expected IsAboveWarningThreshold=true for near-full usage")
	}
	if !state.IsAboveErrorThreshold {
		t.Error("expected IsAboveErrorThreshold=true for near-full usage")
	}
	if !state.IsAtBlockingLimit {
		t.Error("expected IsAtBlockingLimit=true for near-full usage")
	}
}

func TestIsAutoCompactEnabled_DefaultOff(t *testing.T) {
	// Clear all env vars that could affect the result for hermetic test
	t.Setenv("CLAUDE_FEATURE_AUTO_COMPACT", "")
	t.Setenv("DISABLE_COMPACT", "")
	t.Setenv("DISABLE_AUTO_COMPACT", "")
	// Without setting env or flag, auto-compact should be off
	enabled := IsAutoCompactEnabled()
	if enabled {
		t.Error("expected auto-compact to be disabled by default")
	}
}

func TestIsAutoCompactEnabled_DisableCompactEnv(t *testing.T) {
	t.Setenv("DISABLE_COMPACT", "1")
	t.Setenv("CLAUDE_FEATURE_AUTO_COMPACT", "")
	t.Setenv("DISABLE_AUTO_COMPACT", "")

	enabled := IsAutoCompactEnabled()
	if enabled {
		t.Error("expected auto-compact to be disabled when DISABLE_COMPACT=1")
	}
}

func TestIsAutoCompactEnabled_DisableAutoCompactEnv(t *testing.T) {
	t.Setenv("DISABLE_AUTO_COMPACT", "1")
	t.Setenv("CLAUDE_FEATURE_AUTO_COMPACT", "")
	t.Setenv("DISABLE_COMPACT", "")

	enabled := IsAutoCompactEnabled()
	if enabled {
		t.Error("expected auto-compact to be disabled when DISABLE_AUTO_COMPACT=1")
	}
}

func TestMaxConsecutiveFailures(t *testing.T) {
	if MaxConsecutiveFailures != 3 {
		t.Errorf("expected MaxConsecutiveFailures=3, got %d", MaxConsecutiveFailures)
	}
}

func TestTokenWarningState_PercentLeft(t *testing.T) {
	threshold := GetAutoCompactThreshold("claude-sonnet-4-20250514")

	// At 50% of threshold
	halfUsage := threshold / 2
	state := CalculateTokenWarningState(halfUsage, "claude-sonnet-4-20250514")

	if state.PercentLeft <= 0 || state.PercentLeft > 100 {
		t.Errorf("expected percentLeft to be between 1 and 100, got %d", state.PercentLeft)
	}
}

func TestAutoCompactTrackingState_ZeroValue(t *testing.T) {
	var state AutoCompactTrackingState
	if state.Compacted {
		t.Error("expected Compacted=false by default")
	}
	if state.TurnCounter != 0 {
		t.Errorf("expected TurnCounter=0, got %d", state.TurnCounter)
	}
	if state.ConsecutiveFailures != 0 {
		t.Errorf("expected ConsecutiveFailures=0, got %d", state.ConsecutiveFailures)
	}
}

func TestAutoCompactTrackingState_SetValues(t *testing.T) {
	state := AutoCompactTrackingState{
		Compacted:           true,
		TurnCounter:         5,
		TurnID:              "turn-123",
		ConsecutiveFailures: 2,
	}
	if !state.Compacted {
		t.Error("expected Compacted=true")
	}
	if state.TurnCounter != 5 {
		t.Errorf("expected TurnCounter=5, got %d", state.TurnCounter)
	}
	if state.TurnID != "turn-123" {
		t.Errorf("expected TurnID=turn-123, got %s", state.TurnID)
	}
	if state.ConsecutiveFailures != 2 {
		t.Errorf("expected ConsecutiveFailures=2, got %d", state.ConsecutiveFailures)
	}
}

func TestGetMaxOutputTokensForModel_Haiku(t *testing.T) {
	output := GetMaxOutputTokensForModel("claude-haiku-4-20250514")
	if output <= 0 {
		t.Errorf("expected positive output tokens for haiku, got %d", output)
	}
}

func TestGetMaxOutputTokensForModel_Unknown(t *testing.T) {
	output := GetMaxOutputTokensForModel("some-unknown-model")
	if output <= 0 {
		t.Errorf("expected positive output tokens for unknown model, got %d", output)
	}
}
