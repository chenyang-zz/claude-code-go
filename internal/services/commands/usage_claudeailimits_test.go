package commands

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/services/claudeailimits"
)

// fakeClaudeAILimitsStore implements claudeailimits.SettingsStore for tests.
type fakeClaudeAILimitsStore struct {
	saved   map[string]any
	loadVal map[string]any
}

func (f *fakeClaudeAILimitsStore) SaveLastClaudeAILimits(snapshot map[string]any) error {
	f.saved = snapshot
	return nil
}

func (f *fakeClaudeAILimitsStore) GetLastClaudeAILimits() map[string]any {
	return f.loadVal
}

func TestRenderClaudeAILimitsLinesNil(t *testing.T) {
	if got := renderClaudeAILimitsLines(nil); got != nil {
		t.Fatalf("expected nil for nil limits, got %+v", got)
	}
}

func TestRenderClaudeAILimitsLinesIncludesCoreFields(t *testing.T) {
	lines := renderClaudeAILimitsLines(&claudeailimits.ClaudeAILimits{
		Status:                QuotaStatusFromString("rejected"),
		RateLimitType:         claudeailimits.RateLimitFiveHour,
		Utilization:           0.92,
		HasUtilization:        true,
		ResetsAt:              1700000000,
		IsUsingOverage:        true,
		OverageDisabledReason: claudeailimits.OverageOutOfCredits,
		HasSurpassedThreshold: true,
		SurpassedThreshold:    0.9,
	})
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"Claude.ai status: rejected",
		"Representative claim: session limit",
		"Utilization: 92%",
		"Resets at:",
		"Currently using extra usage",
		"Extra usage disabled: out_of_credits",
		"Surpassed threshold: 90%",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected line containing %q in:\n%s", want, joined)
		}
	}
}

// QuotaStatusFromString is a small adapter used so the test can build a
// ClaudeAILimits without re-exporting the package-level constants.
func QuotaStatusFromString(s string) claudeailimits.QuotaStatus {
	return claudeailimits.QuotaStatus(s)
}

func TestRenderUsageSnapshotIncludesPersistedClaudeAILimits(t *testing.T) {
	store := &fakeClaudeAILimitsStore{
		loadVal: map[string]any{
			"status":        "rejected",
			"rateLimitType": "seven_day",
			"utilization":   0.85,
		},
	}
	claudeailimits.SetSettingsStore(store)
	t.Cleanup(func() { claudeailimits.SetSettingsStore(nil) })

	snapshot := UsageLimitsSnapshot{Status: "allowed"}
	output := renderUsageSnapshot(snapshot)
	if !strings.Contains(output, "Claude.ai status: rejected") {
		t.Fatalf("expected persisted Claude.ai status in output, got:\n%s", output)
	}
	if !strings.Contains(output, "weekly limit") {
		t.Fatalf("expected representative claim in output, got:\n%s", output)
	}
}

func TestRenderStatsSnapshotIncludesPersistedClaudeAILimits(t *testing.T) {
	store := &fakeClaudeAILimitsStore{
		loadVal: map[string]any{
			"status": "allowed_warning",
		},
	}
	claudeailimits.SetSettingsStore(store)
	t.Cleanup(func() { claudeailimits.SetSettingsStore(nil) })

	snapshot := UsageLimitsSnapshot{Status: "allowed_warning"}
	output := renderStatsSnapshot(snapshot)
	if !strings.Contains(output, "Claude.ai status: allowed_warning") {
		t.Fatalf("expected persisted Claude.ai status in stats output, got:\n%s", output)
	}
}

func TestRenderUsageSnapshotSkipsClaudeAILimitsWhenNotPersisted(t *testing.T) {
	store := &fakeClaudeAILimitsStore{loadVal: nil}
	claudeailimits.SetSettingsStore(store)
	t.Cleanup(func() { claudeailimits.SetSettingsStore(nil) })

	snapshot := UsageLimitsSnapshot{Status: "allowed"}
	output := renderUsageSnapshot(snapshot)
	if strings.Contains(output, "Claude.ai status:") {
		t.Fatalf("did not expect persisted block when no snapshot exists, got:\n%s", output)
	}
}
