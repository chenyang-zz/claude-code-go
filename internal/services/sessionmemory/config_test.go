package sessionmemory_test

import (
	"context"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/services/sessionmemory"
)

func TestGetSessionMemoryConfig_Defaults(t *testing.T) {
	sessionmemory.ResetState()

	cfg := sessionmemory.GetSessionMemoryConfig()
	if cfg.MinimumMessageTokensToInit != 10000 {
		t.Errorf("MinimumMessageTokensToInit = %d, want 10000", cfg.MinimumMessageTokensToInit)
	}
	if cfg.MinimumTokensBetweenUpdate != 5000 {
		t.Errorf("MinimumTokensBetweenUpdate = %d, want 5000", cfg.MinimumTokensBetweenUpdate)
	}
	if cfg.ToolCallsBetweenUpdates != 3 {
		t.Errorf("ToolCallsBetweenUpdates = %d, want 3", cfg.ToolCallsBetweenUpdates)
	}
}

func TestSetGetSessionMemoryConfig(t *testing.T) {
	sessionmemory.ResetState()

	want := sessionmemory.SessionMemoryConfig{
		MinimumMessageTokensToInit: 20000,
		MinimumTokensBetweenUpdate: 8000,
		ToolCallsBetweenUpdates:    5,
	}
	sessionmemory.SetSessionMemoryConfig(want)

	got := sessionmemory.GetSessionMemoryConfig()
	if got != want {
		t.Errorf("GetSessionMemoryConfig() = %+v, want %+v", got, want)
	}
}

func TestMarkExtractionStartedCompleted(t *testing.T) {
	sessionmemory.ResetState()

	if sessionmemory.IsExtractionInProgress() {
		t.Error("IsExtractionInProgress() = true before MarkExtractionStarted")
	}

	sessionmemory.MarkExtractionStarted()
	if !sessionmemory.IsExtractionInProgress() {
		t.Error("IsExtractionInProgress() = false after MarkExtractionStarted")
	}

	sessionmemory.MarkExtractionCompleted()
	if sessionmemory.IsExtractionInProgress() {
		t.Error("IsExtractionInProgress() = true after MarkExtractionCompleted")
	}
}

func TestWaitForSessionMemoryExtraction_Timeout(t *testing.T) {
	sessionmemory.ResetState()

	sessionmemory.MarkExtractionStarted()

	ctx := context.Background()
	start := time.Now()
	err := sessionmemory.WaitForSessionMemoryExtraction(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("WaitForSessionMemoryExtraction() error = %v, want nil", err)
	}

	// The timeout is 15s. We just check it returns within a reasonable window
	// — at least 1s (one poll cycle) and no more than a generous upper bound.
	if elapsed < 500*time.Millisecond {
		t.Errorf("Wait returned too fast (%v), likely didn't poll", elapsed)
	}
	if elapsed > 20*time.Second {
		t.Errorf("Wait took too long (%v), exceeded expected timeout", elapsed)
	}
}

func TestRecordExtractionTokenCount(t *testing.T) {
	sessionmemory.ResetState()

	if got := sessionmemory.GetTokensAtLastExtraction(); got != 0 {
		t.Fatalf("initial tokens = %d, want 0", got)
	}

	want := 12345
	sessionmemory.RecordExtractionTokenCount(want)

	if got := sessionmemory.GetTokensAtLastExtraction(); got != want {
		t.Errorf("GetTokensAtLastExtraction() = %d, want %d", got, want)
	}
}

func TestMarkSessionMemoryInitialized(t *testing.T) {
	sessionmemory.ResetState()

	if sessionmemory.IsSessionMemoryInitialized() {
		t.Error("IsSessionMemoryInitialized() = true before MarkSessionMemoryInitialized")
	}

	sessionmemory.MarkSessionMemoryInitialized()
	if !sessionmemory.IsSessionMemoryInitialized() {
		t.Error("IsSessionMemoryInitialized() = false after MarkSessionMemoryInitialized")
	}
}

func TestSetGetLastSummarizedMessageID(t *testing.T) {
	sessionmemory.ResetState()

	if got := sessionmemory.GetLastSummarizedMessageID(); got != "" {
		t.Fatalf("initial lastSummarizedMessageID = %q, want empty", got)
	}

	want := "some-uuid-12345"
	sessionmemory.SetLastSummarizedMessageID(want)

	if got := sessionmemory.GetLastSummarizedMessageID(); got != want {
		t.Errorf("GetLastSummarizedMessageID() = %q, want %q", got, want)
	}
}

func TestResetState(t *testing.T) {
	// Step 1: modify various aspects of state.
	sessionmemory.SetSessionMemoryConfig(sessionmemory.SessionMemoryConfig{
		MinimumMessageTokensToInit: 999,
		MinimumTokensBetweenUpdate: 999,
		ToolCallsBetweenUpdates:    999,
	})
	sessionmemory.MarkExtractionStarted()
	sessionmemory.RecordExtractionTokenCount(999)
	sessionmemory.MarkSessionMemoryInitialized()
	sessionmemory.SetLastSummarizedMessageID("non-default")

	// Step 2: reset.
	sessionmemory.ResetState()

	// Step 3: verify everything is back to default.
	cfg := sessionmemory.GetSessionMemoryConfig()
	if cfg.MinimumMessageTokensToInit != 10000 {
		t.Errorf("after ResetState, MinimumMessageTokensToInit = %d, want 10000", cfg.MinimumMessageTokensToInit)
	}
	if cfg.MinimumTokensBetweenUpdate != 5000 {
		t.Errorf("after ResetState, MinimumTokensBetweenUpdate = %d, want 5000", cfg.MinimumTokensBetweenUpdate)
	}
	if cfg.ToolCallsBetweenUpdates != 3 {
		t.Errorf("after ResetState, ToolCallsBetweenUpdates = %d, want 3", cfg.ToolCallsBetweenUpdates)
	}
	if sessionmemory.IsExtractionInProgress() {
		t.Error("IsExtractionInProgress() = true after ResetState")
	}
	if got := sessionmemory.GetTokensAtLastExtraction(); got != 0 {
		t.Errorf("GetTokensAtLastExtraction() = %d, want 0 after ResetState", got)
	}
	if sessionmemory.IsSessionMemoryInitialized() {
		t.Error("IsSessionMemoryInitialized() = true after ResetState")
	}
	if got := sessionmemory.GetLastSummarizedMessageID(); got != "" {
		t.Errorf("GetLastSummarizedMessageID() = %q, want empty after ResetState", got)
	}
}
