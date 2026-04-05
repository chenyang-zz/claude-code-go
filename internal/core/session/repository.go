package session

import (
	"context"
	"errors"
	"time"
)

// ErrSessionNotFound reports that the requested session does not exist in the backing store.
var ErrSessionNotFound = errors.New("session not found")

// Summary carries the minimum recent-session metadata needed for `/resume` discovery output.
type Summary struct {
	// ID identifies one logical session.
	ID string
	// ProjectPath records the workspace the session belongs to.
	ProjectPath string
	// UpdatedAt records when the session snapshot was last overwritten.
	UpdatedAt time.Time
}

// Lookup scopes repository queries that do not target one explicit session identifier.
type Lookup struct {
	// ProjectPath limits the query to one workspace path.
	ProjectPath string
	// Limit bounds recent-session queries.
	Limit int
}

// Repository stores and restores normalized session snapshots.
type Repository interface {
	// Save persists the latest normalized session state.
	Save(ctx context.Context, session Session) error
	// Load restores a previously saved session by identifier.
	Load(ctx context.Context, id string) (Session, error)
	// LoadLatest restores the most recently updated session matching the supplied lookup.
	LoadLatest(ctx context.Context, lookup Lookup) (Session, error)
	// ListRecent returns recent session summaries within one lookup scope.
	ListRecent(ctx context.Context, lookup Lookup) ([]Summary, error)
}
