package autocompact

import (
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

var (
	initMu    sync.Mutex
	globalSvc *AutoCompactService
)

// AutoCompactService provides auto-compact threshold calculations and trigger
// decisions. Currently a stateless utility holder — stateful tracking
// (AutoCompactTrackingState) is managed by the caller (query loop).
type AutoCompactService struct{}

// NewAutoCompactService creates a new AutoCompactService instance.
func NewAutoCompactService() *AutoCompactService {
	return &AutoCompactService{}
}

// InitAutoCompactService creates and registers the global auto-compact service.
// Returns nil when the feature flag is disabled.
func InitAutoCompactService() *AutoCompactService {
	initMu.Lock()
	defer initMu.Unlock()

	if !IsAutoCompactEnabled() {
		logger.DebugCF("autocompact", "skipping init: feature flag disabled", nil)
		globalSvc = nil
		return nil
	}

	svc := NewAutoCompactService()
	globalSvc = svc
	logger.DebugCF("autocompact", "autocompact service initialized", nil)
	return svc
}

// CurrentService returns the active service instance (or nil when
// uninitialised or feature-flagged off).
func CurrentService() *AutoCompactService {
	initMu.Lock()
	defer initMu.Unlock()
	return globalSvc
}
