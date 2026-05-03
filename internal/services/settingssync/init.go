package settingssync

import (
	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// InitOptions captures the wiring inputs required to bootstrap the
// settings sync service.
type InitOptions struct {
	// HomeDir is the user's home directory used for resolving ~/.claude paths.
	HomeDir string
	// Config is the runtime configuration used for OAuth and path resolution.
	Config *config.Config
	// SubscriptionLoader returns the current OAuth tokens. Used by
	// eligibility and HTTP helpers.
	SubscriptionLoader SubscriptionLoader
}

// Init wires the settings sync service with the supplied dependencies.
// Safe to call multiple times — the most recent options win.
func Init(opts InitOptions) {
	SetConfig(opts.Config)
	SetSubscriptionLoader(opts.SubscriptionLoader)

	logger.DebugCF("settingssync", "initialised", map[string]any{
		"home_dir":              opts.HomeDir,
		"config_present":        opts.Config != nil,
		"subscription_loader":   opts.SubscriptionLoader != nil,
		"push_enabled":          IsSettingsSyncPushEnabled(),
		"pull_enabled":          IsSettingsSyncPullEnabled(),
	})
}
