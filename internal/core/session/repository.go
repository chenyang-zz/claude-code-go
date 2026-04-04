package session

import (
	"context"
	"errors"
)

// ErrSessionNotFound reports that the requested session does not exist in the backing store.
var ErrSessionNotFound = errors.New("session not found")

// Repository stores and restores normalized session snapshots.
type Repository interface {
	// Save persists the latest normalized session state.
	Save(ctx context.Context, session Session) error
	// Load restores a previously saved session by identifier.
	Load(ctx context.Context, id string) (Session, error)
}
