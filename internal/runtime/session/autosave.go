package session

import (
	"context"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
)

// AutoSave converts normalized runtime history into persisted session state.
type AutoSave struct {
	// Manager performs the actual save operation.
	Manager *Manager
}

// NewAutoSave builds an autosave helper from a session manager.
func NewAutoSave(manager *Manager) *AutoSave {
	return &AutoSave{Manager: manager}
}

// PersistHistory overwrites the target session with the latest normalized conversation history.
func (a *AutoSave) PersistHistory(ctx context.Context, sessionID string, history conversation.History) (coresession.Snapshot, error) {
	if a == nil || a.Manager == nil {
		return coresession.Snapshot{}, fmt.Errorf("session autosave manager is not configured")
	}

	return a.Manager.ReplaceMessages(ctx, sessionID, history.Messages)
}
