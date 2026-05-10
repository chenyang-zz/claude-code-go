package query

import "time"

// BudgetTracker tracks token budget state across query continuations.
type BudgetTracker struct {
	ContinuationCount  int
	LastDeltaTokens    int
	LastGlobalTurnTokens int
	StartedAt          time.Time
}

// CreateBudgetTracker creates a new BudgetTracker with zeroed counters.
func CreateBudgetTracker() BudgetTracker {
	return BudgetTracker{
		StartedAt: time.Now(),
	}
}

// TokenBudgetDecision is the result of a token budget check.
type TokenBudgetDecision struct {
	// Action is either "continue" or "stop".
	Action string

	// NudgeMessage is the continuation prompt message (continue only).
	NudgeMessage string

	// CompletionEvent summarizes the query budget event (stop only).
	CompletionEvent *BudgetCompletionEvent
}

// BudgetCompletionEvent summarizes the final budget state at query completion.
type BudgetCompletionEvent struct {
	ContinuationCount  int
	Pct                int
	TurnTokens         int
	Budget             int
	DiminishingReturns bool
	DurationMs         int64
}

const (
	completionThreshold = 0.9
	diminishingThreshold = 500
)

// CheckTokenBudget evaluates whether the current query can continue or must stop
// based on token budget usage.
func CheckTokenBudget(tracker *BudgetTracker, agentID string, budget int, globalTurnTokens int) TokenBudgetDecision {
	if agentID != "" || budget <= 0 {
		return TokenBudgetDecision{Action: "stop"}
	}

	turnTokens := globalTurnTokens
	pct := int(float64(turnTokens) / float64(budget) * 100)
	deltaSinceLastCheck := globalTurnTokens - tracker.LastGlobalTurnTokens

	isDiminishing := tracker.ContinuationCount >= 3 &&
		deltaSinceLastCheck < diminishingThreshold &&
		tracker.LastDeltaTokens < diminishingThreshold

	if !isDiminishing && turnTokens < int(float64(budget)*completionThreshold) {
		tracker.ContinuationCount++
		tracker.LastDeltaTokens = deltaSinceLastCheck
		tracker.LastGlobalTurnTokens = globalTurnTokens
		return TokenBudgetDecision{
			Action:        "continue",
			NudgeMessage:  budgetContinuationMessage(pct, turnTokens, budget),
			CompletionEvent: &BudgetCompletionEvent{
				ContinuationCount: tracker.ContinuationCount,
				Pct:              pct,
				TurnTokens:       turnTokens,
				Budget:           budget,
			},
		}
	}

	var completionEvent *BudgetCompletionEvent
	if isDiminishing || tracker.ContinuationCount > 0 {
		completionEvent = &BudgetCompletionEvent{
			ContinuationCount:  tracker.ContinuationCount,
			Pct:               pct,
			TurnTokens:        turnTokens,
			Budget:            budget,
			DiminishingReturns: isDiminishing,
			DurationMs:        time.Since(tracker.StartedAt).Milliseconds(),
		}
	}

	return TokenBudgetDecision{
		Action:          "stop",
		CompletionEvent: completionEvent,
	}
}

func budgetContinuationMessage(pct, turnTokens, budget int) string {
	return formatNudge("Budget continuation", pct, turnTokens, budget)
}

func formatNudge(prefix string, pct, turnTokens, budget int) string {
	return prefix + ": " + itoa(pct) + "% of $" + itoa(budget) + " used (" + itoa(turnTokens) + " tokens)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
