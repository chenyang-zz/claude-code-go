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
	// CustomTitle stores the optional user-assigned session title.
	CustomTitle string
	// Preview carries the minimum human-readable session summary shown by `/resume`.
	Preview string
	// UpdatedAt records when the session snapshot was last overwritten.
	UpdatedAt time.Time
}

// Lookup scopes repository queries that do not target one explicit session identifier.
type Lookup struct {
	// ProjectPath limits the query to one workspace path.
	ProjectPath string
	// AllProjects widens the query to every persisted workspace instead of one project path.
	AllProjects bool
	// Limit bounds recent-session queries.
	Limit int
	// Query carries the minimum free-text search term used by `/resume <search-term>`.
	Query string
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
	// Search returns project-scoped session summaries matching one free-text query.
	Search(ctx context.Context, lookup Lookup) ([]Summary, error)
	// FindByCustomTitle returns session summaries whose custom title exactly matches the supplied lookup query.
	FindByCustomTitle(ctx context.Context, lookup Lookup) ([]Summary, error)
	// UpdateCustomTitle overwrites the user-assigned title for one persisted session.
	UpdateCustomTitle(ctx context.Context, id string, title string) error
}
