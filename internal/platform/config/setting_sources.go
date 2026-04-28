package config

import (
	"fmt"
	"strings"
)

// SettingSource identifies one disk-backed Claude Code settings source understood by the migrated Go host.
type SettingSource string

const (
	// SettingSourceUserSettings identifies the user-scoped `~/.claude/settings.json` source.
	SettingSourceUserSettings SettingSource = "userSettings"
	// SettingSourceProjectSettings identifies the repository-scoped `.claude/settings.json` source.
	SettingSourceProjectSettings SettingSource = "projectSettings"
	// SettingSourceLocalSettings identifies the gitignored `.claude/settings.local.json` source.
	SettingSourceLocalSettings SettingSource = "localSettings"
	// SettingSourceFlagSettings identifies the explicit `--settings` CLI override source.
	SettingSourceFlagSettings SettingSource = "flagSettings"
	// SettingSourcePolicySettings identifies enterprise-managed settings loaded from the managed settings layer.
	SettingSourcePolicySettings SettingSource = "policySettings"
)

// DefaultSettingSources returns the default on-disk settings source order used when no CLI filter is provided.
func DefaultSettingSources() []SettingSource {
	return []SettingSource{
		SettingSourceUserSettings,
		SettingSourceProjectSettings,
		SettingSourceLocalSettings,
	}
}

// ParseSettingSourcesFlag decodes one `--setting-sources` CLI value into the migrated disk-backed source identifiers.
func ParseSettingSourcesFlag(flag string) ([]SettingSource, error) {
	if flag == "" {
		return []SettingSource{}, nil
	}

	parts := strings.Split(flag, ",")
	sources := make([]SettingSource, 0, len(parts))
	for _, part := range parts {
		switch strings.TrimSpace(part) {
		case "user":
			sources = append(sources, SettingSourceUserSettings)
		case "project":
			sources = append(sources, SettingSourceProjectSettings)
		case "local":
			sources = append(sources, SettingSourceLocalSettings)
		default:
			return nil, fmt.Errorf("invalid setting source: %s. valid options are: user, project, local", strings.TrimSpace(part))
		}
	}

	return sources, nil
}

// FormatSettingSourcesFlag encodes parsed disk-backed setting sources back to one canonical CLI flag value.
// The output preserves the provided order and only includes user/project/local entries.
func FormatSettingSourcesFlag(sources []SettingSource) string {
	if len(sources) == 0 {
		return ""
	}

	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		switch source {
		case SettingSourceUserSettings:
			parts = append(parts, "user")
		case SettingSourceProjectSettings:
			parts = append(parts, "project")
		case SettingSourceLocalSettings:
			parts = append(parts, "local")
		}
	}
	return strings.Join(parts, ",")
}
