package tips

import (
	"math"
	"testing"
)

func TestGetSessionsSinceLastShown_NilStore(t *testing.T) {
	SetHistoryStore(nil)
	result := GetSessionsSinceLastShown("any")
	if result != math.MaxInt32 {
		t.Fatalf("expected MaxInt32 for nil store, got %d", result)
	}
}

func TestGetSessionsSinceLastShown_NeverShown(t *testing.T) {
	store := &mockStore{numStartups: 10, history: map[string]int{}}
	SetHistoryStore(store)

	result := GetSessionsSinceLastShown("unknown")
	if result != math.MaxInt32 {
		t.Fatalf("expected MaxInt32 for unseen tip, got %d", result)
	}
}

func TestGetSessionsSinceLastShown_ShownPreviously(t *testing.T) {
	store := &mockStore{
		numStartups: 50,
		history:     map[string]int{"tip-a": 40},
	}
	SetHistoryStore(store)

	result := GetSessionsSinceLastShown("tip-a")
	if result != 10 {
		t.Fatalf("expected 10 sessions since shown, got %d", result)
	}
}

func TestRecordTipShown_NilStore(t *testing.T) {
	SetHistoryStore(nil)
	err := RecordTipShown("tip")
	if err != nil {
		t.Fatalf("expected nil error for nil store, got %v", err)
	}
}

func TestRecordTipShown_UpdatesHistory(t *testing.T) {
	store := &mockStore{numStartups: 42, history: map[string]int{}}
	SetHistoryStore(store)

	err := RecordTipShown("tip-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.history["tip-x"] != 42 {
		t.Fatalf("expected history to record 42, got %d", store.history["tip-x"])
	}
}

func TestIncrementNumStartups_NilStore(t *testing.T) {
	SetHistoryStore(nil)
	err := IncrementNumStartups()
	if err != nil {
		t.Fatalf("expected nil error for nil store, got %v", err)
	}
}

func TestIncrementNumStartups_BumpsCounter(t *testing.T) {
	store := &mockStore{numStartups: 5}
	SetHistoryStore(store)

	err := IncrementNumStartups()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.numStartups != 6 {
		t.Fatalf("expected numStartups=6, got %d", store.numStartups)
	}
}
