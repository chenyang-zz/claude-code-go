package tips

import (
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Init initialises the Tips system.
//
// If spinner tips are not enabled, it returns immediately without registering
// any state.
func Init(store HistoryStore) {
	if !IsSpinnerTipsEnabled() {
		logger.DebugCF("tips", "spinner tips disabled, skipping init", nil)
		return
	}

	SetHistoryStore(store)

	if err := IncrementNumStartups(); err != nil {
		logger.WarnCF("tips", "failed to increment startup counter", map[string]any{
			"error": err.Error(),
		})
	}

	logger.DebugCF("tips", "tips system initialised", nil)
}

// GetTipToShow returns a tip for display, or nil if none is available.
// It is safe to call even when tips are disabled.
func GetTipToShow() *Tip {
	if !IsSpinnerTipsEnabled() {
		return nil
	}
	return GetTipToShowOnSpinner()
}

// OnTipShown records that a tip was displayed.
func OnTipShown(tip *Tip) {
	if tip == nil {
		return
	}
	if err := RecordTipShown(tip.ID); err != nil {
		logger.WarnCF("tips", "failed to record tip shown", map[string]any{
			"tip_id": tip.ID,
			"error":  err.Error(),
		})
	}
}

// TipContext carries the runtime context needed by the REPL to show a tip.
type TipContext struct {
	Tip     *Tip
	ShowTip func()
}

// NewTipContext builds a TipContext for the REPL layer.
// If tips are disabled or no tip is relevant, Tip is nil.
func NewTipContext() TipContext {
	tip := GetTipToShow()
	return TipContext{
		Tip: tip,
		ShowTip: func() {
			OnTipShown(tip)
		},
	}
}
