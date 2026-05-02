package tips

// selectTipWithLongestTimeSinceShown picks the tip that has gone the longest
// without being shown. If no tips are available it returns nil.
func selectTipWithLongestTimeSinceShown(available []Tip) *Tip {
	if len(available) == 0 {
		return nil
	}
	if len(available) == 1 {
		return &available[0]
	}

	var best *Tip
	bestSessions := -1
	for i := range available {
		sessions := GetSessionsSinceLastShown(available[i].ID)
		if sessions > bestSessions {
			bestSessions = sessions
			best = &available[i]
		}
	}
	return best
}

// GetTipToShowOnSpinner returns a tip appropriate for display on the spinner,
// or nil if tips are disabled or no tip is currently relevant.
func GetTipToShowOnSpinner() *Tip {
	if !IsSpinnerTipsEnabled() {
		return nil
	}
	tips := GetRelevantTips()
	if len(tips) == 0 {
		return nil
	}
	return selectTipWithLongestTimeSinceShown(tips)
}
