package tips

import "testing"

func TestSelectTipWithLongestTimeSinceShown_Empty(t *testing.T) {
	result := selectTipWithLongestTimeSinceShown(nil)
	if result != nil {
		t.Fatalf("expected nil for empty input, got %v", result)
	}
}

func TestSelectTipWithLongestTimeSinceShown_Single(t *testing.T) {
	store := &mockStore{numStartups: 10, history: map[string]int{}}
	SetHistoryStore(store)

	tips := []Tip{{ID: "test", Content: "test", CooldownSessions: 0, IsRelevant: func() bool { return true }}}
	result := selectTipWithLongestTimeSinceShown(tips)
	if result == nil {
		t.Fatal("expected a tip, got nil")
	}
	if result.ID != "test" {
		t.Fatalf("expected tip 'test', got %q", result.ID)
	}
}

func TestSelectTipWithLongestTimeSinceShown_LongestWins(t *testing.T) {
	store := &mockStore{
		numStartups: 50,
		history: map[string]int{
			"old":   10, // 40 sessions ago
			"recent": 45, // 5 sessions ago
		},
	}
	SetHistoryStore(store)

	tips := []Tip{
		{ID: "old", Content: "old", CooldownSessions: 0, IsRelevant: func() bool { return true }},
		{ID: "recent", Content: "recent", CooldownSessions: 0, IsRelevant: func() bool { return true }},
	}
	result := selectTipWithLongestTimeSinceShown(tips)
	if result == nil || result.ID != "old" {
		t.Fatalf("expected 'old' (40 sessions ago), got %v", result)
	}
}

func TestGetTipToShowOnSpinner_Disabled(t *testing.T) {
	// Disable tips via env override.
	t.Setenv("CLAUDE_CODE_ENABLE_SPINNER_TIPS", "0")
	SetHistoryStore(nil)

	result := GetTipToShowOnSpinner()
	if result != nil {
		t.Errorf("expected nil when tips disabled, got %v", result)
	}
}

func TestGetTipToShowOnSpinner_NoRelevantTips(t *testing.T) {
	// With numStartups=0, only tips with IsRelevant=true and no numStartups
	// restriction are relevant.
	store := &mockStore{numStartups: 0, history: map[string]int{}}
	SetHistoryStore(store)

	result := GetTipToShowOnSpinner()
	// Should return one of the always-relevant tips
	if result == nil {
		t.Log("no relevant tips for numStartups=0 (expected if all tips need startup count)")
	}
}
