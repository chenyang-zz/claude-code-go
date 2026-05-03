package teammemsync

import (
	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SubscriptionLoader abstracts OAuth token access for eligibility checks.
// Mirrors the shared pattern used by claudeailimits, policylimits, and settingssync.
type SubscriptionLoader interface {
	LoadOAuthTokens() (*struct {
		AccessToken string
		Scopes      []string
	}, error)
}

// InitOptions captures the wiring inputs required to bootstrap the team memory
// sync service.
type InitOptions struct {
	// HomeDir is the user's home directory used for resolving ~/.claude paths.
	HomeDir string
	// Config is the runtime configuration.
	Config *config.Config
	// ProjectRoot is the project root directory used for memory path resolution.
	ProjectRoot string
}

// Init wires the team memory sync service with the supplied dependencies.
// Safe to call multiple times; the most recent options win.
func Init(opts InitOptions) {
	logger.DebugCF("teammemsync", "initialised", map[string]any{
		"home_dir":      opts.HomeDir,
		"project_root":  opts.ProjectRoot,
		"config_present": opts.Config != nil,
		"feature_enabled": IsFeatureEnabled(),
		"team_mem_enabled": IsTeamMemoryEnabled(),
	})
}
