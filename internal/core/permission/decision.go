package permission

// Decision describes the normalized outcome returned by the minimal permission layer.
type Decision string

const (
	// DecisionDeny blocks the requested filesystem operation.
	DecisionDeny Decision = "deny"
	// DecisionAsk requires the caller to request explicit approval before continuing.
	DecisionAsk Decision = "ask"
	// DecisionAllow grants the requested filesystem operation.
	DecisionAllow Decision = "allow"
)

// Valid reports whether the decision is one of the supported permission outcomes.
func (d Decision) Valid() bool {
	switch d {
	case DecisionDeny, DecisionAsk, DecisionAllow:
		return true
	default:
		return false
	}
}

// Evaluation captures the decision returned for one filesystem permission check.
type Evaluation struct {
	// Decision stores the normalized permission outcome.
	Decision Decision
	// Rule points to the matched rule when the decision came from an explicit rule hit.
	Rule *Rule
	// Message stores a stable caller-facing explanation for deny or ask outcomes.
	Message string
}
