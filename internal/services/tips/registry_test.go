package tips

import (
	"runtime"
	"testing"
)

// mockStore is a test double for HistoryStore.
type mockStore struct {
	history     map[string]int
	numStartups int
}

func (m *mockStore) GetTipsHistory() map[string]int { return m.history }
func (m *mockStore) GetNumStartups() int            { return m.numStartups }
func (m *mockStore) RecordTipShown(tipID string) error {
	if m.history == nil {
		m.history = make(map[string]int)
	}
	m.history[tipID] = m.numStartups
	return nil
}
func (m *mockStore) IncrementNumStartups() error {
	m.numStartups++
	return nil
}

func TestGetRelevantTips_AllTipsWhenNoHistory(t *testing.T) {
	store := &mockStore{numStartups: 100}
	SetHistoryStore(store)

	tips := GetRelevantTips()
	// With numStartups=100, tips requiring numStartups < 10 or > 10 are filtered
	// Tips with IsRelevant=true and no cooldown restriction should all appear
	if len(tips) == 0 {
		t.Fatalf("expected some tips, got none")
	}
}

func TestGetRelevantTips_NewUserWarmupOnlyForLowStartups(t *testing.T) {
	store := &mockStore{numStartups: 5}
	SetHistoryStore(store)

	// With numStartups=5, new-user-warmup should be relevant
	tips := GetRelevantTips()
	found := false
	for _, tip := range tips {
		if tip.ID == "new-user-warmup" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected new-user-warmup to be relevant for numStartups=5")
	}

	// With numStartups=20, new-user-warmup should NOT be relevant
	store.numStartups = 20
	tips = GetRelevantTips()
	for _, tip := range tips {
		if tip.ID == "new-user-warmup" {
			t.Errorf("expected new-user-warmup to be irrelevant for numStartups=20")
		}
	}
}

func TestGetRelevantTips_CooldownRespected(t *testing.T) {
	store := &mockStore{
		numStartups: 100,
		history:     map[string]int{"continue": 99}, // shown 1 session ago, cooldown=10
	}
	SetHistoryStore(store)

	tips := GetRelevantTips()
	for _, tip := range tips {
		if tip.ID == "continue" {
			t.Errorf("expected 'continue' to be excluded due to cooldown")
		}
	}
}

func TestGetRelevantTips_CooldownExpired(t *testing.T) {
	store := &mockStore{
		numStartups: 120,
		history:     map[string]int{"continue": 105}, // shown 15 sessions ago, cooldown=10
	}
	SetHistoryStore(store)

	tips := GetRelevantTips()
	found := false
	for _, tip := range tips {
		if tip.ID == "continue" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'continue' to be included after cooldown expired")
	}
}

func TestGetRelevantTips_DesktopAppPlatformCheck(t *testing.T) {
	store := &mockStore{numStartups: 100}
	SetHistoryStore(store)

	// desktop-app is relevant when GOOS != "linux"
	tips := GetRelevantTips()
	found := false
	for _, tip := range tips {
		if tip.ID == "desktop-app" {
			found = true
			break
		}
	}
	want := runtime.GOOS != "linux"
	if found != want {
		t.Errorf("desktop-app found=%v, want=%v (GOOS=%q)", found, want, runtime.GOOS)
	}
}
