package settingssync

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsSettingsSyncPushEnabled reports whether the push (upload) side of settings
// sync is active. Controlled by the CLAUDE_FEATURE_SETTINGS_SYNC_PUSH env var.
func IsSettingsSyncPushEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagSettingsSyncPush)
}

// IsSettingsSyncPullEnabled reports whether the pull (download) side of settings
// sync is active. Controlled by the CLAUDE_FEATURE_SETTINGS_SYNC_PULL env var.
func IsSettingsSyncPullEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagSettingsSyncPull)
}
