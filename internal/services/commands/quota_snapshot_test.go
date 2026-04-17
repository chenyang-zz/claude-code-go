package commands

import (
	"context"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// stubUsageLimitsProber returns one fixed snapshot for command-layer tests.
type stubUsageLimitsProber struct {
	snapshot UsageLimitsSnapshot
}

// ProbeUsage returns the configured fixed snapshot.
func (s stubUsageLimitsProber) ProbeUsage(ctx context.Context, cfg coreconfig.Config) UsageLimitsSnapshot {
	_ = ctx
	_ = cfg
	return s.snapshot
}
