package datetimeparser

import (
	"sync"

	"github.com/sheepzhao/claude-code-go/internal/services/haiku"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// InitOptions configures InitDateTimeParser.
type InitOptions struct {
	// Querier is the haiku service that will dispatch parser requests.
	// Required; pass haiku.CurrentService() when called from bootstrap.
	Querier haiku.Querier
}

var (
	currentMu      sync.RWMutex
	currentSvcImpl *Service
)

// InitDateTimeParser registers a date/time parser service backed by the
// given haiku querier and returns it. Stored as the package-level singleton
// consumed by Parse.
//
// Returns nil and resets the singleton when:
//   - FlagDateTimeParser is disabled (default), or
//   - opts.Querier is nil (haiku service was not initialised).
func InitDateTimeParser(opts InitOptions) *Service {
	if !IsDateTimeParserEnabled() {
		logger.DebugCF("datetime_parser", "skipping init: feature flag disabled", nil)
		setCurrentService(nil)
		return nil
	}
	if opts.Querier == nil {
		logger.DebugCF("datetime_parser", "skipping init: haiku querier nil", nil)
		setCurrentService(nil)
		return nil
	}
	svc := NewService(opts.Querier)
	setCurrentService(svc)
	logger.DebugCF("datetime_parser", "datetime parser initialized", nil)
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
