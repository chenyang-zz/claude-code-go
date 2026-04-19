package compact

import (
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestGetContextWindowForModel_Default(t *testing.T) {
	got := GetContextWindowForModel("unknown-model")
	if got != ModelContextWindowDefault {
		t.Fatalf("expected %d for unknown model, got %d", ModelContextWindowDefault, got)
	}
}

func TestGetContextWindowForModel_KnownModels(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		{"claude-sonnet-4-20250514", 200_000},
		{"claude-opus-4-6", 200_000},
		{"claude-haiku-4-5-20251001", 200_000},
	}
	for _, tc := range tests {
		got := GetContextWindowForModel(tc.model)
		if got != tc.expected {
			t.Errorf("GetContextWindowForModel(%q) = %d, want %d", tc.model, got, tc.expected)
		}
	}
}

func TestGetContextWindowForModel_EnvOverride(t *testing.T) {
	os.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "50000")
	defer os.Unsetenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW")

	got := GetContextWindowForModel("claude-sonnet-4-20250514")
	if got != 50_000 {
		t.Fatalf("expected 50000 with env override, got %d", got)
	}
}

func TestGetContextWindowForModel_EnvOverrideInvalid(t *testing.T) {
	os.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "not-a-number")
	defer os.Unsetenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW")

	got := GetContextWindowForModel("claude-sonnet-4-20250514")
	if got != 200_000 {
		t.Fatalf("expected 200000 with invalid env override, got %d", got)
	}
}

func TestGetEffectiveContextWindowSize(t *testing.T) {
	// effective = contextWindow - min(maxOutputForModel, 20000)
	// = 200000 - 20000 = 180000
	got := GetEffectiveContextWindowSize("claude-sonnet-4-20250514")
	expected := 200_000 - MaxOutputTokensForSummary
	if got != expected {
		t.Fatalf("expected %d, got %d", expected, got)
	}
}

func TestGetAutoCompactThreshold(t *testing.T) {
	// threshold = effective - 13000
	// = 180000 - 13000 = 167000
	got := GetAutoCompactThreshold("claude-sonnet-4-20250514")
	effective := GetEffectiveContextWindowSize("claude-sonnet-4-20250514")
	expected := effective - AutoCompactBufferTokens
	if got != expected {
		t.Fatalf("expected %d, got %d", expected, got)
	}
}

func TestShouldAutoCompact_BelowThreshold(t *testing.T) {
	// Clear env vars to ensure auto-compact is enabled
	os.Unsetenv("DISABLE_COMPACT")
	os.Unsetenv("DISABLE_AUTO_COMPACT")

	// Create a small message that won't exceed threshold
	messages := createTestMessages(100) // ~100 chars → ~25 tokens
	got := ShouldAutoCompact(messages, "claude-sonnet-4-20250514")
	if got {
		t.Fatal("expected auto-compact NOT to trigger for small messages")
	}
}

func TestShouldAutoCompact_DisabledByEnv(t *testing.T) {
	os.Setenv("DISABLE_COMPACT", "1")
	defer os.Unsetenv("DISABLE_COMPACT")

	messages := createTestMessages(100000) // Very large
	got := ShouldAutoCompact(messages, "claude-sonnet-4-20250514")
	if got {
		t.Fatal("expected auto-compact disabled when DISABLE_COMPACT=1")
	}
}

func TestShouldAutoCompact_DisabledByAutoCompactEnv(t *testing.T) {
	os.Unsetenv("DISABLE_COMPACT")
	os.Setenv("DISABLE_AUTO_COMPACT", "true")
	defer os.Unsetenv("DISABLE_AUTO_COMPACT")

	messages := createTestMessages(100000)
	got := ShouldAutoCompact(messages, "claude-sonnet-4-20250514")
	if got {
		t.Fatal("expected auto-compact disabled when DISABLE_AUTO_COMPACT=true")
	}
}

func TestShouldAutoCompactWithTracking_CircuitBreaker(t *testing.T) {
	os.Unsetenv("DISABLE_COMPACT")
	os.Unsetenv("DISABLE_AUTO_COMPACT")

	tracking := &TrackingState{
		ConsecutiveFailures: MaxConsecutiveAutoCompactFailures,
	}
	// Even with large messages, circuit breaker should prevent triggering.
	messages := createTestMessages(1000000)
	got := ShouldAutoCompactWithTracking(messages, "claude-sonnet-4-20250514", tracking)
	if got {
		t.Fatal("expected circuit breaker to prevent auto-compact")
	}
}

func TestIsAutoCompactEnabled_Default(t *testing.T) {
	os.Unsetenv("DISABLE_COMPACT")
	os.Unsetenv("DISABLE_AUTO_COMPACT")
	if !IsAutoCompactEnabled() {
		t.Fatal("expected auto-compact enabled by default")
	}
}

func TestIsAutoCompactEnabled_DisabledCompact(t *testing.T) {
	os.Setenv("DISABLE_COMPACT", "1")
	defer os.Unsetenv("DISABLE_COMPACT")
	if IsAutoCompactEnabled() {
		t.Fatal("expected auto-compact disabled with DISABLE_COMPACT=1")
	}
}

func TestIsAutoCompactEnabled_DisabledAutoCompact(t *testing.T) {
	os.Unsetenv("DISABLE_COMPACT")
	os.Setenv("DISABLE_AUTO_COMPACT", "yes")
	defer os.Unsetenv("DISABLE_AUTO_COMPACT")
	if IsAutoCompactEnabled() {
		t.Fatal("expected auto-compact disabled with DISABLE_AUTO_COMPACT=yes")
	}
}

// createTestMessages builds a slice of messages with approximately totalChars
// characters of text content.
func createTestMessages(totalChars int) []message.Message {
	var msgs []message.Message
	remaining := totalChars
	chunkSize := 100
	for remaining > 0 {
		size := chunkSize
		if remaining < chunkSize {
			size = remaining
		}
		text := make([]byte, size)
		for i := range text {
			text[i] = 'x'
		}
		msgs = append(msgs, message.Message{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart(string(text)),
			},
		})
		remaining -= size
	}
	return msgs
}
