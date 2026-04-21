package engine

import (
	"testing"
)

func TestParseTokenBudget_ShortHandStart(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"+500k fix the bug", 500_000},
		{"+1.5m analyze this", 1_500_000},
		{"  +2b big task", 2_000_000_000},
		{"+100K", 100_000},
		{"+3M tokens", 3_000_000},
	}
	for _, tt := range tests {
		got, ok := ParseTokenBudget(tt.input)
		if !ok {
			t.Errorf("ParseTokenBudget(%q): expected match, got false", tt.input)
		}
		if got != tt.expected {
			t.Errorf("ParseTokenBudget(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestParseTokenBudget_ShortHandEnd(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"fix the bug +500k", 500_000},
		{"do something +1.5m.", 1_500_000},
		{"analyze +2b!", 2_000_000_000},
	}
	for _, tt := range tests {
		got, ok := ParseTokenBudget(tt.input)
		if !ok {
			t.Errorf("ParseTokenBudget(%q): expected match, got false", tt.input)
		}
		if got != tt.expected {
			t.Errorf("ParseTokenBudget(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestParseTokenBudget_Verbose(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"use 2M tokens to analyze", 2_000_000},
		{"spend 500k tokens", 500_000},
		{"please use 1.5b tokens for this", 1_500_000_000},
		{"Use 100K tokens", 100_000},
	}
	for _, tt := range tests {
		got, ok := ParseTokenBudget(tt.input)
		if !ok {
			t.Errorf("ParseTokenBudget(%q): expected match, got false", tt.input)
		}
		if got != tt.expected {
			t.Errorf("ParseTokenBudget(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestParseTokenBudget_NoMatch(t *testing.T) {
	tests := []string{
		"fix the bug",
		"500k tokens",    // no + prefix or use/spend
		"the price is $500k",
		"+500 tokens",    // no k/m/b suffix
		"",
	}
	for _, input := range tests {
		got, ok := ParseTokenBudget(input)
		if ok {
			t.Errorf("ParseTokenBudget(%q): expected no match, got %d", input, got)
		}
	}
}

func TestParseTokenBudget_ShorthandStartTakesPrecedence(t *testing.T) {
	// When input is just "+500k", both start and end would match,
	// but start is checked first.
	got, ok := ParseTokenBudget("+500k")
	if !ok {
		t.Error("expected match for '+500k'")
	}
	if got != 500_000 {
		t.Errorf("ParseTokenBudget('+500k') = %d, want 500000", got)
	}
}

func TestFormatBudgetNudgeMessage(t *testing.T) {
	msg := FormatBudgetNudgeMessage(50, 5000, 10000)
	expected := "Stopped at 50% of token target (5,000 / 10,000). Keep working \u2014 do not summarize."
	if msg != expected {
		t.Errorf("FormatBudgetNudgeMessage() = %q, want %q", msg, expected)
	}
}

func TestFormatBudgetNudgeMessage_LargeNumbers(t *testing.T) {
	msg := FormatBudgetNudgeMessage(75, 1500000, 2000000)
	expected := "Stopped at 75% of token target (1,500,000 / 2,000,000). Keep working \u2014 do not summarize."
	if msg != expected {
		t.Errorf("FormatBudgetNudgeMessage() = %q, want %q", msg, expected)
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{9999, "9,999"},
		{10000, "10,000"},
		{100000, "100,000"},
		{1000000, "1,000,000"},
		{500000, "500,000"},
	}
	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.expected {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestComputeTaskBudgetRemaining_InitialCompute(t *testing.T) {
	// When no previous remaining (0), uses total as starting point.
	remaining := ComputeTaskBudgetRemaining(0, 1_000_000, 800_000, 200_000)
	// remaining starts at total (1M), subtracts 800K + 200K = 1M → 0
	if remaining != 0 {
		t.Errorf("ComputeTaskBudgetRemaining(0, 1M, 800K, 200K) = %d, want 0", remaining)
	}
}

func TestComputeTaskBudgetRemaining_SubsequentCompute(t *testing.T) {
	// Second compaction: remaining starts at 500K, subtracts context.
	remaining := ComputeTaskBudgetRemaining(500_000, 1_000_000, 300_000, 100_000)
	// 500K - (300K + 100K) = 100K
	if remaining != 100_000 {
		t.Errorf("ComputeTaskBudgetRemaining(500K, 1M, 300K, 100K) = %d, want 100000", remaining)
	}
}

func TestComputeTaskBudgetRemaining_FloorAtZero(t *testing.T) {
	remaining := ComputeTaskBudgetRemaining(100, 1_000_000, 800_000, 200_000)
	if remaining != 0 {
		t.Errorf("ComputeTaskBudgetRemaining should floor at 0, got %d", remaining)
	}
}

func TestComputeTaskBudgetRemaining_UsesActualSummaryRequestUsage(t *testing.T) {
	remaining := ComputeTaskBudgetRemaining(0, 1_000_000, 250_000, 10_000)
	if remaining != 740_000 {
		t.Errorf("ComputeTaskBudgetRemaining(0, 1M, 250K, 10K) = %d, want 740000", remaining)
	}
}
