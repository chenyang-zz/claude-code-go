package haiku

import (
	"context"
)

// DefaultHaikuModel is the small/fast model used when QueryParams.Model is
// empty. Mirrors awaysummary.DefaultModel for cross-service consistency.
const DefaultHaikuModel = "claude-haiku-4-5-20251001"

// DefaultMaxOutputTokens caps Haiku output unless the caller overrides it.
// Haiku helper call sites (toolusesummary, sessionTitle, shellPrefix, etc.)
// only need a handful of tokens, so we keep the default low to bound spend
// and latency. Override via QueryParams.MaxOutputTokens when more is needed.
const DefaultMaxOutputTokens = 2048

// QueryParams is the input bundle for a single Haiku query.
type QueryParams struct {
	// SystemPrompt is the system message handed to the model.
	SystemPrompt string
	// UserPrompt is the user-turn text content.
	UserPrompt string
	// Model overrides DefaultHaikuModel when non-empty.
	Model string
	// MaxOutputTokens overrides DefaultMaxOutputTokens when non-zero.
	MaxOutputTokens int
	// EnablePromptCaching adds a cache_control marker to the last content
	// block of the request when true. Mirrors TS queryHaiku's
	// enablePromptCaching flag (default false at the Haiku layer; callers
	// like toolusesummary explicitly opt in).
	EnablePromptCaching bool
	// IsNonInteractiveSession is propagated from the calling session for
	// future analytics/log correlation. The current Go runtime only logs the
	// value; no behavioural difference.
	IsNonInteractiveSession bool
	// QuerySource is a free-form label identifying the call site
	// ("tool_use_summary_generation", "session_title", ...). Logged for
	// debugging.
	QuerySource string
}

// QueryResult is the aggregated output returned by Service.Query.
type QueryResult struct {
	// Text is the trimmed assistant text content.
	Text string
	// StopReason is the stop_reason field from the final stream event.
	StopReason string
	// Usage carries token accounting from the model response.
	Usage Usage
}

// Usage mirrors model.Usage for callers that should not depend on the
// model package directly.
type Usage struct {
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

// Querier is the minimal interface downstream services depend on. Service
// implements it; tests can substitute a stub. Keeping the surface small
// avoids leaking the rest of Service through the dependency boundary.
type Querier interface {
	Query(ctx context.Context, params QueryParams) (*QueryResult, error)
}

// Query dispatches the request through the active package-level service.
// Returns ErrClientUnavailable when the service has not been initialised.
// Equivalent to currentService().Query(ctx, params) for callers that prefer
// a function-style entry point.
func Query(ctx context.Context, params QueryParams) (*QueryResult, error) {
	svc := currentService()
	if svc == nil {
		return nil, ErrClientUnavailable
	}
	return svc.Query(ctx, params)
}
