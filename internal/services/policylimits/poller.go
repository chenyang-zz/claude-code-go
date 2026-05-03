package policylimits

import (
	"context"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	defaultPollingInterval = 1 * time.Hour
)

var (
	pollerMu       sync.Mutex
	pollerInterval time.Duration
	pollerCancel   context.CancelFunc
	pollerRunning  bool
)

// StartPolling begins background polling for policy limit changes.
// If a poller is already running it is a no-op. The interval defaults
// to 1 hour and can be overridden by SetPollingInterval.
func StartPolling(ctx context.Context) {
	pollerMu.Lock()
	defer pollerMu.Unlock()
	if pollerRunning {
		return
	}
	if !IsEligible() {
		return
	}

	interval := pollerInterval
	if interval <= 0 {
		interval = defaultPollingInterval
	}

	var pollerCtx context.Context
	pollerCtx, pollerCancel = context.WithCancel(ctx)
	pollerRunning = true

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-pollerCtx.Done():
				return
			case <-ticker.C:
				logger.DebugCF("policylimits", "background poll triggered", nil)
				_, _ = fetchAndLoad(pollerCtx)
			}
		}
	}()
}

// StopPolling stops the background poller.
func StopPolling() {
	pollerMu.Lock()
	defer pollerMu.Unlock()
	if !pollerRunning {
		return
	}
	if pollerCancel != nil {
		pollerCancel()
	}
	pollerRunning = false
}

// SetPollingInterval overrides the default polling interval. Must be called
// before StartPolling; calling it while polling is running has no effect until
// the next StartPolling call.
func SetPollingInterval(d time.Duration) {
	pollerMu.Lock()
	defer pollerMu.Unlock()
	pollerInterval = d
}
