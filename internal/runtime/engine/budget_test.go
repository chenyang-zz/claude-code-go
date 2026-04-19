package engine

import (
	"testing"
)

func TestCheckTokenBudget_SubAgentStops(t *testing.T) {
	tracker := NewBudgetTracker()
	decision := checkTokenBudget(&tracker, "sub-agent-id", 10000, 1000)
	if decision.Action != "stop" {
		t.Fatalf("expected stop for sub-agent, got %s", decision.Action)
	}
	if decision.CompletionEvent != nil {
		t.Fatal("expected no completion event for sub-agent")
	}
}

func TestCheckTokenBudget_NoBudgetStops(t *testing.T) {
	tracker := NewBudgetTracker()
	decision := checkTokenBudget(&tracker, "", 0, 1000)
	if decision.Action != "stop" {
		t.Fatalf("expected stop with no budget, got %s", decision.Action)
	}
}

func TestCheckTokenBudget_ContinueWhenUnderThreshold(t *testing.T) {
	tracker := NewBudgetTracker()
	// 5000 / 10000 = 50% < 90%, should continue
	decision := checkTokenBudget(&tracker, "", 10000, 5000)
	if decision.Action != "continue" {
		t.Fatalf("expected continue at 50%%, got %s", decision.Action)
	}
	if decision.NudgeMessage == "" {
		t.Fatal("expected nudge message")
	}
	if decision.ContinuationCount != 1 {
		t.Fatalf("expected continuation count 1, got %d", decision.ContinuationCount)
	}
	if tracker.ContinuationCount != 1 {
		t.Fatalf("expected tracker continuation count 1, got %d", tracker.ContinuationCount)
	}
	if tracker.LastDeltaTokens != 5000 {
		t.Fatalf("expected last delta 5000, got %d", tracker.LastDeltaTokens)
	}
}

func TestCheckTokenBudget_StopAtThreshold(t *testing.T) {
	tracker := NewBudgetTracker()
	// First continue to build up state
	checkTokenBudget(&tracker, "", 10000, 5000)
	// Now at 95% — above 90% threshold
	decision := checkTokenBudget(&tracker, "", 10000, 9500)
	if decision.Action != "stop" {
		t.Fatalf("expected stop at 95%%, got %s", decision.Action)
	}
	if decision.CompletionEvent == nil {
		t.Fatal("expected completion event")
	}
	if decision.CompletionEvent.DiminishingReturns {
		t.Fatal("should not be diminishing at threshold")
	}
}

func TestCheckTokenBudget_DiminishingReturns(t *testing.T) {
	tracker := NewBudgetTracker()
	budget := 100000
	// Build up 3 continuations, each producing small deltas
	tokens := 1000
	for i := 0; i < 3; i++ {
		decision := checkTokenBudget(&tracker, "", budget, tokens)
		if decision.Action != "continue" {
			t.Fatalf("iteration %d: expected continue, got %s", i, decision.Action)
		}
		tokens += 100 // Each continuation produces only 100 tokens (very small)
	}
	// 4th check: tracker.ContinuationCount == 3, deltaSinceLastCheck == 100, LastDeltaTokens == 100
	// Both < 500 → diminishing returns should trigger
	decision := checkTokenBudget(&tracker, "", budget, tokens)
	if decision.Action != "stop" {
		t.Fatalf("expected stop for diminishing returns, got %s", decision.Action)
	}
	if decision.CompletionEvent == nil {
		t.Fatal("expected completion event")
	}
	if !decision.CompletionEvent.DiminishingReturns {
		t.Fatal("expected diminishing returns to be true")
	}
}

func TestCheckTokenBudget_NoDiminishingWhenLargeDelta(t *testing.T) {
	tracker := NewBudgetTracker()
	budget := 100000
	// Build up 3 continuations with large deltas
	tokens := 1000
	for i := 0; i < 3; i++ {
		tokens += 10000 // Each continuation produces 10000 tokens (large)
		decision := checkTokenBudget(&tracker, "", budget, tokens)
		if decision.Action != "continue" {
			t.Fatalf("iteration %d: expected continue, got %s", i, decision.Action)
		}
	}
	// 4th check: still producing > 500 tokens per interval → not diminishing
	decision := checkTokenBudget(&tracker, "", budget, tokens+10000)
	if decision.Action == "stop" && decision.CompletionEvent != nil && decision.CompletionEvent.DiminishingReturns {
		t.Fatal("should not be diminishing when delta is large")
	}
}

func TestCheckTokenBudget_TrackerStateUpdates(t *testing.T) {
	tracker := NewBudgetTracker()
	// First check at 2000 tokens
	checkTokenBudget(&tracker, "", 10000, 2000)
	if tracker.LastGlobalTurnTokens != 2000 {
		t.Fatalf("expected lastGlobalTurnTokens 2000, got %d", tracker.LastGlobalTurnTokens)
	}
	// Second check at 5000 tokens
	checkTokenBudget(&tracker, "", 10000, 5000)
	if tracker.LastGlobalTurnTokens != 5000 {
		t.Fatalf("expected lastGlobalTurnTokens 5000, got %d", tracker.LastGlobalTurnTokens)
	}
	if tracker.LastDeltaTokens != 3000 {
		t.Fatalf("expected lastDeltaTokens 3000, got %d", tracker.LastDeltaTokens)
	}
}
