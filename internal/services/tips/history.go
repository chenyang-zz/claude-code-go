package tips

import "math"

// HistoryReader provides read access to the tip-show history.
type HistoryReader interface {
	GetTipsHistory() map[string]int
	GetNumStartups() int
}

// HistoryWriter provides write access to the tip-show history.
type HistoryWriter interface {
	RecordTipShown(tipID string) error
	IncrementNumStartups() error
}

// HistoryStore combines read and write operations.
type HistoryStore interface {
	HistoryReader
	HistoryWriter
}

// defaultStore is the package-level history store set by Init.
var defaultStore HistoryStore

// SetHistoryStore configures the history store used by the tips package.
// It is called once during bootstrap.
func SetHistoryStore(store HistoryStore) {
	defaultStore = store
}

// GetSessionsSinceLastShown returns how many startup sessions have passed
// since the tip was last shown. If the tip has never been shown, it returns
// math.MaxInt32 (standing in for Infinity).
func GetSessionsSinceLastShown(tipID string) int {
	if defaultStore == nil {
		return math.MaxInt32
	}
	history := defaultStore.GetTipsHistory()
	lastShown, ok := history[tipID]
	if !ok {
		return math.MaxInt32
	}
	current := defaultStore.GetNumStartups()
	diff := current - lastShown
	if diff < 0 {
		return 0
	}
	return diff
}

// RecordTipShown marks a tip as shown in the current session.
func RecordTipShown(tipID string) error {
	if defaultStore == nil {
		return nil
	}
	return defaultStore.RecordTipShown(tipID)
}

// IncrementNumStartups bumps the startup counter. It should be called once
// per process startup before any tip selection happens.
func IncrementNumStartups() error {
	if defaultStore == nil {
		return nil
	}
	return defaultStore.IncrementNumStartups()
}
