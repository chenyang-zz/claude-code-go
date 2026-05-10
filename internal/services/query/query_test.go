package query

import (
	"testing"
)

func TestBuildQueryConfig(t *testing.T) {
	cfg := BuildQueryConfig("test-session-1")

	if cfg.SessionID != "test-session-1" {
		t.Errorf("expected session ID 'test-session-1', got %q", cfg.SessionID)
	}

	if !cfg.Gates.FastModeEnabled {
		t.Error("expected FastModeEnabled to default to true")
	}
}

func TestProductionDeps(t *testing.T) {
	deps := ProductionDeps()

	if deps.UUID == nil {
		t.Fatal("expected UUID func to be non-nil")
	}

	uuid1 := deps.UUID()
	uuid2 := deps.UUID()

	if uuid1 == uuid2 {
		t.Error("expected consecutive UUID calls to return different values")
	}

	if len(uuid1) == 0 {
		t.Error("expected non-empty UUID string")
	}
}

func TestCreateBudgetTracker(t *testing.T) {
	tracker := CreateBudgetTracker()

	if tracker.ContinuationCount != 0 {
		t.Errorf("expected ContinuationCount 0, got %d", tracker.ContinuationCount)
	}
	if tracker.LastDeltaTokens != 0 {
		t.Errorf("expected LastDeltaTokens 0, got %d", tracker.LastDeltaTokens)
	}
	if tracker.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
}

func TestCheckTokenBudgetNoBudget(t *testing.T) {
	tracker := CreateBudgetTracker()

	// No budget (budget <= 0) should always stop
	decision := CheckTokenBudget(&tracker, "", 0, 1000)
	if decision.Action != "stop" {
		t.Errorf("expected stop action for zero budget, got %q", decision.Action)
	}
	if decision.CompletionEvent != nil {
		t.Error("expected nil completion event when no budget was set")
	}

	// With agent ID should also stop
	decision = CheckTokenBudget(&tracker, "agent-1", 10000, 1000)
	if decision.Action != "stop" {
		t.Errorf("expected stop action for agent query, got %q", decision.Action)
	}
}

func TestCheckTokenBudgetContinue(t *testing.T) {
	tracker := CreateBudgetTracker()

	// Under 90% threshold should continue
	decision := CheckTokenBudget(&tracker, "", 10000, 1000)
	if decision.Action != "continue" {
		t.Errorf("expected continue at 10%%, got %q", decision.Action)
	}

	if tracker.ContinuationCount != 1 {
		t.Errorf("expected ContinuationCount 1, got %d", tracker.ContinuationCount)
	}
}

func TestCheckTokenBudgetStopAtThreshold(t *testing.T) {
	tracker := CreateBudgetTracker()

	// Over 90% threshold should stop
	decision := CheckTokenBudget(&tracker, "", 10000, 9500)
	if decision.Action != "stop" {
		t.Errorf("expected stop at 95%%, got %q", decision.Action)
	}
}

func TestCheckTokenBudgetDiminishingReturns(t *testing.T) {
	tracker := CreateBudgetTracker()

	// Simulate 3+ continuations with small deltas
	tracker.ContinuationCount = 3
	tracker.LastDeltaTokens = 100
	tracker.LastGlobalTurnTokens = 100

	decision := CheckTokenBudget(&tracker, "", 100000, 200)
	if decision.Action != "stop" {
		t.Errorf("expected stop due to diminishing returns, got %q", decision.Action)
	}
	if decision.CompletionEvent == nil || !decision.CompletionEvent.DiminishingReturns {
		t.Error("expected diminishing returns flag in completion event")
	}
}
