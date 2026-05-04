package haiku

import (
	"sync"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// InitOptions configures InitHaiku. Currently only the model client is
// required; future fields can carry alternative defaults (custom max tokens,
// metric sinks) without a signature break.
type InitOptions struct {
	Client model.Client
}

var (
	currentMu      sync.RWMutex
	currentSvcImpl *Service
)

// InitHaiku registers a Haiku service backed by the given model client and
// returns it. The service is stored as the package-level singleton consumed
// by Query and downstream services.
//
// Returns nil when FlagHaikuQuery is explicitly disabled or when opts.Client
// is nil. The singleton is reset to nil in those cases so a stale prior
// service does not linger across reinitialisation in tests.
func InitHaiku(opts InitOptions) *Service {
	if !IsHaikuEnabled() {
		logger.DebugCF("haiku", "skipping init: feature flag disabled", nil)
		setCurrentService(nil)
		return nil
	}
	if opts.Client == nil {
		logger.DebugCF("haiku", "skipping init: model client nil", nil)
		setCurrentService(nil)
		return nil
	}
	svc := NewService(opts.Client)
	setCurrentService(svc)
	logger.DebugCF("haiku", "haiku initialized", map[string]any{
		"default_model": DefaultHaikuModel,
	})
	return svc
}

// IsInitialized reports whether the package-level Haiku service is wired.
func IsInitialized() bool {
	return currentService() != nil
}

// CurrentService returns the active Haiku service (or nil when uninitialised).
// Exposed primarily so bootstrap and downstream services can pass the same
// instance into toolusesummary.InitOptions.
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
