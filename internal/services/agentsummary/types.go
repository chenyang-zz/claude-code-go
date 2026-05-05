package agentsummary

import (
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// SummaryStore is a thread-safe store for agent summary text keyed by task ID.
// Mirrors the TS updateAgentSummary function in LocalAgentTask.tsx but without
// React AppState coupling.
type SummaryStore struct {
	mu   sync.RWMutex
	data map[string]string // taskID -> summary text
}

// NewSummaryStore creates a new SummaryStore.
func NewSummaryStore() *SummaryStore {
	return &SummaryStore{
		data: make(map[string]string),
	}
}

// Store sets the summary text for the given task ID.
func (s *SummaryStore) Store(taskID, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[taskID] = summary
}

// Load returns the summary text for the given task ID and whether it exists.
func (s *SummaryStore) Load(taskID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summary, ok := s.data[taskID]
	return summary, ok
}

// Delete removes the summary for the given task ID.
func (s *SummaryStore) Delete(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, taskID)
}

// StopFunc stops a periodic agent summarization task.
type StopFunc func()

// SummaryConfig controls the periodic agent summary behavior.
type SummaryConfig struct {
	// Interval is the period between summary generations (default 30s).
	Interval time.Duration
}

// DefaultSummaryConfig returns the default summary configuration.
func DefaultSummaryConfig() SummaryConfig {
	return SummaryConfig{
		Interval: 30 * time.Second,
	}
}

// GetMessagesFunc returns the current messages for an agent.
// This replaces the TS getAgentTranscript call by providing direct access
// to the agent's live message history.
type GetMessagesFunc func() []message.Message
