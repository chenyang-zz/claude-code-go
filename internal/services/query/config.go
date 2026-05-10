package query

// QueryConfig holds immutable configuration snapshotted at query() entry.
// Separating config from per-iteration state makes future step() extraction
// tractable — a pure reducer can take (state, event, config) where config is
// plain data.
type QueryConfig struct {
	// SessionID identifies the logical conversation for this query.
	SessionID string

	// Gates holds runtime feature gates (env/statsig). NOT feature() gates —
	// those are tree-shaking boundaries in TS and inline at the guarded blocks.
	Gates QueryGates
}

// QueryGates collects runtime toggle values that affect query behavior.
type QueryGates struct {
	// StreamingToolExecution enables streaming tool execution feedback.
	StreamingToolExecution bool
	// EmitToolUseSummaries controls whether tool-use summaries are emitted.
	EmitToolUseSummaries bool
	// IsAnt marks internal Anthro build mode.
	IsAnt bool
	// FastModeEnabled controls fast mode (defaults to true).
	FastModeEnabled bool
}

// BuildQueryConfig creates a QueryConfig from the current environment/session.
func BuildQueryConfig(sessionID string) QueryConfig {
	return QueryConfig{
		SessionID: sessionID,
		Gates: QueryGates{
			StreamingToolExecution: false, // gates.statig — default off
			EmitToolUseSummaries:   false,
			IsAnt:                  false,
			FastModeEnabled:        true,
		},
	}
}
