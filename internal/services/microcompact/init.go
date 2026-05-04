package microcompact

import (
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

var (
	initMu    sync.Mutex
	globalSvc *MicrocompactService
)

// InitMicrocompactService creates and registers the global microcompact service.
// Returns nil and resets the singleton when the feature flag is disabled.
func InitMicrocompactService() *MicrocompactService {
	initMu.Lock()
	defer initMu.Unlock()

	if !IsMicroCompactEnabled() {
		logger.DebugCF("microcompact", "skipping init: feature flag disabled", nil)
		globalSvc = nil
		return nil
	}

	svc := NewMicrocompactService()
	globalSvc = svc
	logger.DebugCF("microcompact", "microcompact initialized", nil)
	return svc
}

// CurrentService returns the active service (or nil when uninitialised).
func CurrentService() *MicrocompactService {
	initMu.Lock()
	defer initMu.Unlock()
	return globalSvc
}
