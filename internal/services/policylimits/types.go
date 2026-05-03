package policylimits

// PolicyAction is a known organizational policy restriction identifier.
type PolicyAction string

const (
	// ActionAllowRemoteSessions gates remote session features (triggers,
	// teleport, scheduled agents).
	ActionAllowRemoteSessions PolicyAction = "allow_remote_sessions"
	// ActionAllowProductFeedback gates the /feedback command.
	ActionAllowProductFeedback PolicyAction = "allow_product_feedback"
)

// Restriction represents a single policy restriction entry.
type Restriction struct {
	Allowed bool `json:"allowed"`
}

// PolicyLimitsResponse mirrors the Anthropic policy limits API response body.
// Only blocked policies are included; if a policy key is absent, it's allowed.
type PolicyLimitsResponse struct {
	Restrictions map[string]Restriction `json:"restrictions"`
}

// FetchResult is the outcome of a single policy limits fetch attempt.
type FetchResult struct {
	Success      bool
	Restrictions map[string]Restriction // nil means 304 Not Modified (cache is valid)
	ETag         string
	Error        string
	SkipRetry    bool // If true, don't retry on failure (e.g. auth errors)
}
