package policylimits

import (
	"context"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/config"
)

func TestStartPolling_StopPolling(t *testing.T) {
	SetPollingInterval(100 * time.Millisecond)
	defer SetPollingInterval(0)

	// Make eligible so poller starts
	SetConfig(&config.Config{Provider: "anthropic", APIKey: "key"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	StartPolling(ctx)
	if !isPollerRunning() {
		t.Error("poller should be running")
	}

	StopPolling()
	if isPollerRunning() {
		t.Error("poller should be stopped")
	}
}

func TestStartPolling_DoubleStart(t *testing.T) {
	SetPollingInterval(100 * time.Millisecond)
	defer SetPollingInterval(0)

	SetConfig(&config.Config{Provider: "anthropic", APIKey: "key"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	StartPolling(ctx)
	StartPolling(ctx) // double start should be no-op

	StopPolling()
}

func TestWaitForLoad_NotEligible(t *testing.T) {
	SetConfig(nil)
	// Should return immediately without blocking
	done := make(chan struct{})
	go func() {
		WaitForLoad()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Error("WaitForLoad should not block when not eligible")
	}
}

func TestInitializeLoadingPromise(t *testing.T) {
	SetConfig(&config.Config{Provider: "anthropic", APIKey: "key"})
	InitializeLoadingPromise()

	done := make(chan struct{})
	go func() {
		WaitForLoad()
		close(done)
	}()

	// Promise should resolve via timeout or explicit resolution
	select {
	case <-done:
		// OK
	case <-time.After(35 * time.Second):
		t.Error("WaitForLoad should resolve within timeout")
	}
}

func isPollerRunning() bool {
	pollerMu.Lock()
	defer pollerMu.Unlock()
	return pollerRunning
}
