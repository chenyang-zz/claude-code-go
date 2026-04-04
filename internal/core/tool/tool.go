package tool

import "context"

// Tool defines the minimal executable capability exposed to the host runtime.
type Tool interface {
	// Name returns the stable identifier used during registration and dispatch.
	Name() string
	// Description returns a short human-readable summary for the tool.
	Description() string
	// InputSchema returns the declared input contract exposed to provider tool schemas.
	InputSchema() InputSchema
	// IsReadOnly reports whether the tool avoids mutating external state.
	IsReadOnly() bool
	// IsConcurrencySafe reports whether multiple invocations can run in parallel safely.
	IsConcurrencySafe() bool
	// Invoke executes the tool with the provided call envelope and returns a structured result.
	Invoke(ctx context.Context, call Call) (Result, error)
}
