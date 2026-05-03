package teammemsync

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsFeatureEnabled reports whether the team memory sync feature flag is
// enabled. This is the flag-only check used by consumers that have already
// validated the higher-level IsTeamMemoryEnabled gate.
func IsFeatureEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagTeamMemorySync)
}

// IsScannerEnabled reports whether the team memory secret scanner is enabled.
// When enabled, files are scanned against gitleaks rules before push.
func IsScannerEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagTeamMemoryScanner)
}

// IsWatcherEnabled reports whether the team memory file watcher is enabled.
// When enabled, the team memory directory is watched for file changes.
func IsWatcherEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagTeamMemoryWatcher)
}
