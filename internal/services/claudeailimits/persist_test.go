package claudeailimits

import (
	"errors"
	"testing"
)

// fakeStore implements SettingsStore for tests.
type fakeStore struct {
	saved   map[string]any
	loadVal map[string]any
	saveErr error
}

func (f *fakeStore) SaveLastClaudeAILimits(snapshot map[string]any) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = snapshot
	return nil
}

func (f *fakeStore) GetLastClaudeAILimits() map[string]any {
	return f.loadVal
}

func TestSetSettingsStore(t *testing.T) {
	store := &fakeStore{}
	SetSettingsStore(store)
	t.Cleanup(func() { SetSettingsStore(nil) })

	if got := getSettingsStore(); got != store {
		t.Fatalf("getSettingsStore returned wrong instance")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	store := &fakeStore{}
	SetSettingsStore(store)
	t.Cleanup(func() { SetSettingsStore(nil) })

	limits := &ClaudeAILimits{
		Status:                            QuotaStatusAllowedWarning,
		UnifiedRateLimitFallbackAvailable: true,
		ResetsAt:                          1700000000,
		RateLimitType:                     RateLimitSevenDay,
		Utilization:                       0.78,
		HasUtilization:                    true,
		OverageStatus:                     QuotaStatusAllowed,
		OverageResetsAt:                   1700100000,
		OverageDisabledReason:             OverageOutOfCredits,
		IsUsingOverage:                    false,
		SurpassedThreshold:                0.75,
		HasSurpassedThreshold:             true,
		CachedExtraUsageDisabledReason:    OverageMemberLevelDisabled,
	}

	if err := SaveClaudeAILimits(limits); err != nil {
		t.Fatalf("SaveClaudeAILimits returned %v", err)
	}
	if store.saved == nil {
		t.Fatal("expected saved snapshot")
	}

	// Round-trip via decodeLimits.
	store.loadVal = store.saved
	loaded, err := LoadClaudeAILimits()
	if err != nil {
		t.Fatalf("LoadClaudeAILimits returned %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil loaded snapshot")
	}
	if loaded.Status != QuotaStatusAllowedWarning ||
		loaded.RateLimitType != RateLimitSevenDay ||
		!loaded.HasUtilization || loaded.Utilization != 0.78 ||
		loaded.OverageDisabledReason != OverageOutOfCredits ||
		!loaded.HasSurpassedThreshold || loaded.SurpassedThreshold != 0.75 ||
		loaded.CachedExtraUsageDisabledReason != OverageMemberLevelDisabled {
		t.Fatalf("round-trip mismatch: %+v", loaded)
	}
}

func TestSaveNilSnapshotClearsEntry(t *testing.T) {
	store := &fakeStore{saved: map[string]any{"placeholder": true}}
	SetSettingsStore(store)
	t.Cleanup(func() { SetSettingsStore(nil) })

	if err := SaveClaudeAILimits(nil); err != nil {
		t.Fatalf("SaveClaudeAILimits(nil) returned %v", err)
	}
	if store.saved != nil {
		t.Fatalf("expected nil saved snapshot, got %+v", store.saved)
	}
}

func TestSaveSurfacesStoreError(t *testing.T) {
	expected := errors.New("disk full")
	store := &fakeStore{saveErr: expected}
	SetSettingsStore(store)
	t.Cleanup(func() { SetSettingsStore(nil) })

	if err := SaveClaudeAILimits(&ClaudeAILimits{}); !errors.Is(err, expected) {
		t.Fatalf("expected %v, got %v", expected, err)
	}
}

func TestLoadReturnsNilWhenNoStore(t *testing.T) {
	SetSettingsStore(nil)
	got, err := LoadClaudeAILimits()
	if err != nil {
		t.Fatalf("LoadClaudeAILimits returned %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestLoadReturnsNilWhenSnapshotAbsent(t *testing.T) {
	store := &fakeStore{loadVal: nil}
	SetSettingsStore(store)
	t.Cleanup(func() { SetSettingsStore(nil) })

	got, err := LoadClaudeAILimits()
	if err != nil {
		t.Fatalf("LoadClaudeAILimits returned %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestSaveNoOpWhenFlagDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_CLAUDEAI_LIMITS", "0")
	store := &fakeStore{}
	SetSettingsStore(store)
	t.Cleanup(func() { SetSettingsStore(nil) })

	if err := SaveClaudeAILimits(&ClaudeAILimits{Status: QuotaStatusRejected}); err != nil {
		t.Fatalf("SaveClaudeAILimits returned %v", err)
	}
	if store.saved != nil {
		t.Fatalf("expected save to be skipped, got %+v", store.saved)
	}
}

func TestLoadNoOpWhenFlagDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_CLAUDEAI_LIMITS", "0")
	store := &fakeStore{loadVal: map[string]any{"status": "rejected"}}
	SetSettingsStore(store)
	t.Cleanup(func() { SetSettingsStore(nil) })

	got, err := LoadClaudeAILimits()
	if err != nil {
		t.Fatalf("LoadClaudeAILimits returned %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil when flag disabled, got %+v", got)
	}
}

func TestEncodeDecodeOmitsEmptyFields(t *testing.T) {
	encoded := encodeLimits(&ClaudeAILimits{Status: QuotaStatusAllowed})
	if _, ok := encoded["resetsAt"]; ok {
		t.Fatal("resetsAt should be omitted when zero")
	}
	if _, ok := encoded["utilization"]; ok {
		t.Fatal("utilization should be omitted when not present")
	}
	if _, ok := encoded["surpassedThreshold"]; ok {
		t.Fatal("surpassedThreshold should be omitted when not present")
	}
	decoded := decodeLimits(encoded)
	if decoded.Status != QuotaStatusAllowed {
		t.Fatalf("Status round-trip mismatch: %+v", decoded)
	}
}

func TestNumberToInt64(t *testing.T) {
	cases := []struct {
		in   any
		want int64
	}{
		{in: float64(42), want: 42},
		{in: int(7), want: 7},
		{in: int64(99), want: 99},
		{in: nil, want: 0},
		{in: "12", want: 0},
	}
	for _, tc := range cases {
		if got := numberToInt64(tc.in); got != tc.want {
			t.Fatalf("numberToInt64(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestIsClaudeAILimitsEnabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_CLAUDEAI_LIMITS", "")
	if !IsClaudeAILimitsEnabled() {
		t.Fatal("default should be enabled")
	}
	t.Setenv("CLAUDE_FEATURE_CLAUDEAI_LIMITS", "0")
	if IsClaudeAILimitsEnabled() {
		t.Fatal("0 should disable")
	}
	t.Setenv("CLAUDE_FEATURE_CLAUDEAI_LIMITS", "1")
	if !IsClaudeAILimitsEnabled() {
		t.Fatal("1 should leave enabled")
	}
}
