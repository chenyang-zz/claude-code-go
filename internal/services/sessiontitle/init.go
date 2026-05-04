package sessiontitle

import (
	"sync"

	"github.com/sheepzhao/claude-code-go/internal/services/haiku"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// InitOptions configures InitSessionTitle.
type InitOptions struct {
	// Querier is the haiku service that will dispatch title requests.
	// Required; pass haiku.CurrentService() when called from bootstrap.
	Querier haiku.Querier
}

var (
	currentMu      sync.RWMutex
	currentSvcImpl *Service
)

// InitSessionTitle registers a session title service backed by the
// given haiku querier and returns it. Stored as the package-level singleton
// consumed by Generate.
//
// Returns nil and resets the singleton when:
//   - FlagSessionTitle is disabled (default), or
//   - opts.Querier is nil (haiku service was not initialised).
func InitSessionTitle(opts InitOptions) *Service {
	if !IsSessionTitleEnabled() {
		logger.DebugCF("session_title", "skipping init: feature flag disabled", nil)
		setCurrentService(nil)
		return nil
	}
	if opts.Querier == nil {
		logger.DebugCF("session_title", "skipping init: haiku querier nil", nil)
		setCurrentService(nil)
		return nil
	}
	svc := NewService(opts.Querier)
	setCurrentService(svc)
	logger.DebugCF("session_title", "session title initialized", nil)
	return svc
}

// IsInitialized reports whether the package-level service is wired.
func IsInitialized() bool {
	return currentService() != nil
}

// CurrentService returns the active service (or nil when uninitialised).
func CurrentService() *Service {
	return currentService()
}

func currentService() *Service {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return currentSvcImpl
}

func setCurrentService(svc *Service) {
	currentMu.Lock()
	defer currentMu.Unlock()
	currentSvcImpl = svc
}
