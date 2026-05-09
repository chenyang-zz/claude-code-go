package growthbook

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/platform/analytics"
)

func testEmitter(t *testing.T) *analytics.Emitter {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return analytics.NewEmitter(analytics.NewConsoleSink(log), 100)
}

func TestNewExposureLogger(t *testing.T) {
	emitter := testEmitter(t)
	defer emitter.Close()

	logger := NewExposureLogger(emitter)
	if logger == nil {
		t.Fatal("NewExposureLogger returned nil")
	}

	_, ok := logger.(*exposureBridge)
	if !ok {
		t.Fatal("NewExposureLogger did not return *exposureBridge")
	}
}

func TestNewExposureLoggerNilEmitter(t *testing.T) {
	logger := NewExposureLogger(nil)
	if logger == nil {
		t.Fatal("NewExposureLogger(nil) returned nil")
	}
}

func TestExposureBridge_LogExposure(t *testing.T) {
	emitter := testEmitter(t)
	defer emitter.Close()

	bridge := &exposureBridge{emitter: emitter}
	bridge.LogExposure("test-feature", StoredExperimentData{
		ExperimentID: "exp-123",
		VariationID:  1,
		InExperiment: true,
	}, UserAttributes{
		ID:              "user-1",
		DeviceID:        "dev-1",
		SessionID:       "sess-1",
		Platform:        "darwin",
		OrganizationUUID: "org-1",
		AccountUUID:     "acct-1",
	})

	// The event is emitted asynchronously via Emitter.EmitRaw.
	// We cannot easily assert on the console output, but the test verifies
	// that no panic or error occurs during emission.
}

func TestExposureBridge_LogExposureNilEmitter(t *testing.T) {
	bridge := &exposureBridge{emitter: nil}

	// Should not panic
	bridge.LogExposure("test", StoredExperimentData{}, UserAttributes{})
}

func TestExposureBridge_LogExposureAllFields(t *testing.T) {
	emitter := testEmitter(t)
	defer emitter.Close()

	bridge := &exposureBridge{emitter: emitter}

	// Exercise all fields of StoredExperimentData and UserAttributes
	bridge.LogExposure("feature-42", StoredExperimentData{
		ExperimentID:  "exp-99",
		VariationID:   7,
		InExperiment:  true,
		HashAttribute: "id",
		HashValue:     "hash-abc",
	}, UserAttributes{
		ID:               "user-99",
		SessionID:        "sess-99",
		DeviceID:         "dev-99",
		Platform:         "linux",
		OrganizationUUID: "org-99",
		AccountUUID:      "acct-99",
	})

	// Small delay to allow async emission
	time.Sleep(10 * time.Millisecond)
}
