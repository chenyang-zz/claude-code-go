package analytics

import (
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewLoggerDisabled(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{Enabled: false})
	if logger != nil {
		t.Fatal("expected nil when disabled")
	}
}

func TestNewLoggerEnabled(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-1",
	})
	if logger == nil {
		t.Fatal("expected non-nil when enabled")
	}
	defer logger.Shutdown(0)

	if logger.cfg.SessionID != "sess-1" {
		t.Errorf("expected session id sess-1, got %s", logger.cfg.SessionID)
	}
	if logger.cfg.ScheduledDelay != defaultLogsExportInterval {
		t.Errorf("expected default delay %v, got %v", defaultLogsExportInterval, logger.cfg.ScheduledDelay)
	}
}

func TestLogEvent(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-log",
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Shutdown(0)

	ok := logger.LogEvent("test.event", "evt-1", EventMetadata{}, nil, nil)
	if !ok {
		t.Error("expected LogEvent to return true")
	}
}

func TestLogEventWithMetadata(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-meta",
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Shutdown(0)

	meta := EventMetadata{
		Model:  "claude-opus-4",
		Betas:  "beta1",
		UserID: "user-123",
	}
	userMeta := map[string]any{"role": "admin"}
	eventMeta := map[string]any{"source": "test"}

	ok := logger.LogEvent("test.meta", "evt-2", meta, userMeta, eventMeta)
	if !ok {
		t.Error("expected LogEvent to return true")
	}
}

func TestLogEventNilLogger(t *testing.T) {
	var logger *FirstPartyEventLogger
	ok := logger.LogEvent("test.event", "evt-1", EventMetadata{}, nil, nil)
	if ok {
		t.Error("expected false for nil logger")
	}
}

func TestLogEventKillSwitch(t *testing.T) {
	var killed atomic.Bool
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-kill",
		IsKilled:  func() bool { return killed.Load() },
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Shutdown(0)

	killed.Store(true)
	ok := logger.LogEvent("test.event", "evt-1", EventMetadata{}, nil, nil)
	if ok {
		t.Error("expected false when kill switch active")
	}
}

func TestLogEventAfterClose(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-close",
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	logger.Shutdown(0)

	ok := logger.LogEvent("test.event", "evt-1", EventMetadata{}, nil, nil)
	if ok {
		t.Error("expected false after shutdown")
	}

	// Shutdown again should not panic
	logger.Shutdown(100 * time.Millisecond)
}

func TestLogGrowthBookExperiment(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-gb",
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Shutdown(0)

	data := GrowthBookExperimentData{
		ExperimentID: "exp-1",
		VariationID:  1,
	}
	ok := logger.LogGrowthBookExperiment(data)
	if !ok {
		t.Error("expected LogGrowthBookExperiment to return true")
	}
}

func TestLogGrowthBookExperimentWithAllFields(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-gb2",
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Shutdown(0)

	data := GrowthBookExperimentData{
		ExperimentID:        "exp-2",
		VariationID:         2,
		deviceID:            "dev-1",
		accountUUID:         "acct-1",
		organizationUUID:    "org-1",
		SessionID:           "sess-gb2",
		UserAttributes:      map[string]any{"country": "us"},
		ExperimentMetadata:  map[string]any{"source": "web"},
	}
	ok := logger.LogGrowthBookExperiment(data)
	if !ok {
		t.Error("expected LogGrowthBookExperiment to return true")
	}
}

func TestLogGrowthBookExperimentNilLogger(t *testing.T) {
	var logger *FirstPartyEventLogger
	ok := logger.LogGrowthBookExperiment(GrowthBookExperimentData{})
	if ok {
		t.Error("expected false for nil logger")
	}
}

func TestLogGrowthBookExperimentKillSwitch(t *testing.T) {
	var killed atomic.Bool
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-gb-kill",
		IsKilled:  func() bool { return killed.Load() },
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Shutdown(0)

	killed.Store(true)
	ok := logger.LogGrowthBookExperiment(GrowthBookExperimentData{})
	if ok {
		t.Error("expected false when kill switch active")
	}
}

func TestEnqueueNonBlocking(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-nb",
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Shutdown(0)

	// enqueue should not block even when called many times
	allOk := true
	for range 100 {
		if !logger.enqueue(FirstPartyEventLoggingEvent{
			EventType: "test",
			EventData: map[string]any{"n": 0},
		}) {
			allOk = false
		}
	}
	if !allOk {
		t.Error("expected enqueue to accept events (non-blocking)")
	}
}

func TestEnqueueAfterClose(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-close-enq",
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	logger.Shutdown(0)

	ok := logger.enqueue(FirstPartyEventLoggingEvent{EventType: "closed"})
	if ok {
		t.Error("expected enqueue to return false after shutdown")
	}
}

func TestLoggerShutdownWithTimeout(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-timeout",
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	// Add some events before shutdown
	logger.LogEvent("test.event", "evt-1", EventMetadata{}, nil, nil)
	logger.LogEvent("test.event", "evt-2", EventMetadata{}, nil, nil)

	// Shutdown with 1 second timeout - should flush events
	logger.Shutdown(time.Second)
}

func TestLoggerShutdownWaitsIndefinitely(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-wait",
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	logger.LogEvent("test.event", "evt-1", EventMetadata{}, nil, nil)

	done := make(chan struct{})
	go func() {
		logger.Shutdown(0) // wait indefinitely
		close(done)
	}()

	select {
	case <-done:
		// OK - shutdown completed
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown(0) did not complete in 2 seconds")
	}
}

func TestLoggerShutdownNilLogger(t *testing.T) {
	var logger *FirstPartyEventLogger
	err := logger.Shutdown(time.Second)
	if err != nil {
		t.Fatalf("expected nil error for nil logger, got %v", err)
	}
}

func TestLoggerDebugLogging(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	_ = logger

	// Create logger that logs to the debug writer
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	l := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-debug",
		Logger:    discardLogger,
	})
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	l.Shutdown(0)
}

func TestLogEventNilMetadata(t *testing.T) {
	logger := NewFirstPartyEventLogger(FirstPartyEventLoggerConfig{
		Enabled:   true,
		SessionID: "sess-nil",
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Shutdown(0)

	ok := logger.LogEvent("test.event", "evt-1", EventMetadata{}, nil, nil)
	if !ok {
		t.Error("expected LogEvent to return true")
	}
}

func TestMetadataToMap(t *testing.T) {
	m := EventMetadata{
		Model:  "opus",
		Betas:  "b1",
		UserID: "u1",
	}
	result := m.toMap()
	if result["model"] != "opus" {
		t.Errorf("expected model=opus, got %v", result["model"])
	}
	if result["betas"] != "b1" {
		t.Errorf("expected betas=b1, got %v", result["betas"])
	}
	if result["user_id"] != "u1" {
		t.Errorf("expected user_id=u1, got %v", result["user_id"])
	}

	// Empty metadata should produce empty map
	empty := EventMetadata{}.toMap()
	if len(empty) != 0 {
		t.Errorf("expected empty map, got %v", empty)
	}
}
