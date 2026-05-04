package sessionmemorycompact

import (
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

var (
	initMu    sync.Mutex
	globalSvc *SessionMemoryCompactService
)

// SessionMemoryCompactService provides session-memory-based compaction
// logic. Currently a stateless utility holder.
type SessionMemoryCompactService struct{}

// NewSessionMemoryCompactService creates a new SessionMemoryCompactService.
func NewSessionMemoryCompactService() *SessionMemoryCompactService {
	return &SessionMemoryCompactService{}
}

// InitSessionMemoryCompactService creates and registers the global session
// memory compact service. Returns nil when the feature flag is disabled.
func InitSessionMemoryCompactService() *SessionMemoryCompactService {
	initMu.Lock()
	defer initMu.Unlock()

	if !IsSessionMemoryCompactEnabled() {
		logger.DebugCF("sessionmemorycompact", "skipping init: feature flag disabled", nil)
		globalSvc = nil
		return nil
	}

	svc := NewSessionMemoryCompactService()
	globalSvc = svc
	logger.DebugCF("sessionmemorycompact", "session memory compact service initialized", nil)
	return svc
}

// CurrentService returns the active service (or nil when uninitialised).
func CurrentService() *SessionMemoryCompactService {
	initMu.Lock()
	defer initMu.Unlock()
	return globalSvc
}
