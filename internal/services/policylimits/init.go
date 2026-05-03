package policylimits

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// InitOptions captures the wiring inputs required to bootstrap the
// policy limits service.
type InitOptions struct {
	// HomeDir is the user's home directory used for cache file paths.
	HomeDir string
	// Config is the runtime configuration used for eligibility checks.
	Config *config.Config
	// SubscriptionLoader returns the current OAuth tokens. Used by
	// eligibility and fetch helpers.
	SubscriptionLoader SubscriptionLoader
}

// Init wires the policy limits service with the supplied dependencies.
// Safe to call multiple times — the most recent options win.
func Init(opts InitOptions) {
	SetCacheHomeDir(opts.HomeDir)
	SetConfig(opts.Config)
	SetSubscriptionLoader(opts.SubscriptionLoader)

	logger.DebugCF("policylimits", "initialised", map[string]any{
		"home_dir":             opts.HomeDir,
		"config_present":         opts.Config != nil,
		"subscription_loader":    opts.SubscriptionLoader != nil,
		"feature_flag_enabled":   IsPolicyLimitsEnabled(),
	})

	// Early promise creation so downstream systems can wait even before
	// the background load starts.
	InitializeLoadingPromise()

	// Fire-and-forget initial load + background polling.
	go LoadAndStart(context.Background())
}
