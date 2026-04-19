package engine

import (
	"fmt"
	"time"
)

// completionThreshold is the fraction of the token budget that must be reached
// before the engine stops issuing budget continuations.
const completionThreshold = 0.9

// diminishingThreshold is the minimum number of tokens a continuation interval
// must produce to not be considered "diminishing returns".
const diminishingThreshold = 500

// diminishingMinCount is the minimum number of continuations that must have
// occurred before diminishing-returns detection kicks in.
const diminishingMinCount = 3

// BudgetTracker tracks state for the token budget continuation decision loop.
type BudgetTracker struct {
	// ContinuationCount is how many times the system has auto-continued the model.
	ContinuationCount int
	// LastDeltaTokens is the token delta produced in the previous continuation interval.
	LastDeltaTokens int
	// LastGlobalTurnTokens is the cumulative turn output tokens at the last check.
	LastGlobalTurnTokens int
	// StartedAt is the timestamp when the tracker was created.
	StartedAt time.Time
}

// NewBudgetTracker creates a BudgetTracker initialised at the current time.
func NewBudgetTracker() BudgetTracker {
	return BudgetTracker{
		StartedAt: time.Now(),
	}
}

// TokenBudgetDecision represents the outcome of a budget check.
type TokenBudgetDecision struct {
	// Action is either "continue" or "stop".
	Action string
	// NudgeMessage is the continuation prompt injected when action is "continue".
	NudgeMessage string
	// ContinuationCount is the updated continuation count after this decision.
	ContinuationCount int
	// Pct is the percentage of the budget consumed (0-100).
	Pct int
	// CompletionEvent holds telemetry data when the budget concludes, or nil.
	CompletionEvent *BudgetCompletionEvent
}

// BudgetCompletionEvent captures telemetry when the budget loop ends.
type BudgetCompletionEvent struct {
	Pct                int
	Tokens             int
	Budget             int
	DurationMs         int64
	DiminishingReturns bool
}

// checkTokenBudget decides whether the model should be continued or stopped
// based on the token budget and production rate.
//
// Decision flow:
//  1. Immediate stop (no event) if agentID is set (sub-agent) or budget <= 0
//  2. Diminishing returns: ≥3 continuations AND last two intervals each <500 tokens
//  3. Continue: not diminishing AND turn tokens < 90% of budget
//  4. Stop with event: diminishing or previously continued
//  5. Fallback stop: no event
func checkTokenBudget(
	tracker *BudgetTracker,
	agentID string,
	budget int,
	globalTurnTokens int,
) TokenBudgetDecision {
	// Sub-agents and sessions without a budget never trigger budget continuation.
	if agentID != "" || budget <= 0 {
		return TokenBudgetDecision{Action: "stop"}
	}

	deltaSinceLastCheck := globalTurnTokens - tracker.LastGlobalTurnTokens

	// Diminishing-returns detection: the model has already been continued at
	// least 3 times AND both the current and previous intervals produced
	// fewer than 500 tokens.
	isDiminishing := tracker.ContinuationCount >= diminishingMinCount &&
		deltaSinceLastCheck < diminishingThreshold &&
		tracker.LastDeltaTokens < diminishingThreshold

	pct := 0
	if budget > 0 {
		pct = globalTurnTokens * 100 / budget
	}

	// Continue: the model has not yet reached 90% and is not producing
	// diminishing returns.
	if !isDiminishing && globalTurnTokens < int(float64(budget)*completionThreshold) {
		newCount := tracker.ContinuationCount + 1
		tracker.ContinuationCount = newCount
		tracker.LastDeltaTokens = deltaSinceLastCheck
		tracker.LastGlobalTurnTokens = globalTurnTokens

		nudge := fmt.Sprintf(
			"Stopped at %d%% of token target (%d / %d). Keep working — do not summarize.",
			pct, globalTurnTokens, budget,
		)
		return TokenBudgetDecision{
			Action:            "continue",
			NudgeMessage:      nudge,
			ContinuationCount: newCount,
			Pct:               pct,
		}
	}

	// Stop with completion event when the model was previously continued.
	if tracker.ContinuationCount > 0 || isDiminishing {
		return TokenBudgetDecision{
			Action: "stop",
			CompletionEvent: &BudgetCompletionEvent{
				Pct:                pct,
				Tokens:             globalTurnTokens,
				Budget:             budget,
				DurationMs:         time.Since(tracker.StartedAt).Milliseconds(),
				DiminishingReturns: isDiminishing,
			},
		}
	}

	return TokenBudgetDecision{Action: "stop"}
}
