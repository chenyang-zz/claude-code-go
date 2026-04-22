package config

import (
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
)

// Config stores the minimal runtime configuration currently consumed by the Go host.
type Config struct {
	// ProjectPath identifies the current workspace path used for project-scoped runtime behavior.
	ProjectPath string
	// HomeDir stores the resolved user home directory used for config and data file paths.
	HomeDir string
	// Env stores the filtered settings-sourced environment variables that remain eligible for runtime config resolution and bootstrap application.
	Env map[string]string
	// Model overrides the default model selection when provided.
	Model string
	// EffortLevel stores the persisted model effort preference when explicitly configured.
	EffortLevel string
	// HasEffortLevelSetting reports whether EffortLevel was explicitly set by the merged config sources.
	HasEffortLevelSetting bool
	// FastMode stores the persisted fast-mode preference when explicitly configured.
	FastMode bool
	// HasFastModeSetting reports whether FastMode was explicitly set by the merged config sources.
	HasFastModeSetting bool
	// Theme stores the persisted terminal theme preference.
	Theme string
	// EditorMode stores the persisted prompt editor keybinding mode.
	EditorMode string
	// Provider selects which backend provider implementation to use.
	Provider string
	// APIKey carries the credential required by the selected provider.
	APIKey string
	// AuthToken carries the Anthropic bearer token used by first-party account auth.
	AuthToken string
	// APIBaseURL optionally overrides the provider API endpoint.
	APIBaseURL string
	// LoadedSettingSources lists the migrated settings layers that actually participated in config loading.
	LoadedSettingSources []string
	// PolicySettings stores the minimum managed-settings source metadata surfaced by `/status`.
	PolicySettings PolicySettingsConfig
	// APIKeySource stores the environment key that supplied the effective API key when one is configured.
	APIKeySource string
	// AuthTokenSource stores the environment key that supplied the effective auth token when one is configured.
	AuthTokenSource string
	// OAuthAccount stores the minimum cached Claude account metadata surfaced to `/status`.
	OAuthAccount OAuthAccountConfig
	// APIBaseURLSource stores the environment key that supplied the effective API base URL override.
	APIBaseURLSource string
	// ProxyURL stores the effective outbound proxy URL resolved from runtime environment variables.
	ProxyURL string
	// ProxySource stores the environment key that supplied the effective outbound proxy URL.
	ProxySource string
	// AdditionalCACertsPath stores the optional CA bundle path appended to the runtime trust store.
	AdditionalCACertsPath string
	// AdditionalCACertsSource stores the environment key that supplied AdditionalCACertsPath.
	AdditionalCACertsSource string
	// MTLSClientCertPath stores the optional client certificate path used for outbound mTLS.
	MTLSClientCertPath string
	// MTLSClientCertSource stores the environment key that supplied MTLSClientCertPath.
	MTLSClientCertSource string
	// MTLSClientKeyPath stores the optional client private key path used for outbound mTLS.
	MTLSClientKeyPath string
	// MTLSClientKeySource stores the environment key that supplied MTLSClientKeyPath.
	MTLSClientKeySource string
	// ApprovalMode controls the runtime approval behavior.
	ApprovalMode string
	// SessionDBPath points at the session persistence database when enabled.
	SessionDBPath string
	// RemoteSession stores the minimum remote-mode context surfaced by bootstrap.
	RemoteSession RemoteSessionConfig
	// Permissions stores the migrated read-only permission settings surfaced to slash commands.
	Permissions PermissionConfig
	// Hooks stores the hook configuration loaded from settings, keyed by event name.
	Hooks hook.HooksConfig
	// DisableAllHooks disables all hook execution when set via policy settings.
	DisableAllHooks bool
	// OutputFormat selects the output rendering mode (e.g. "console" or "stream-json").
	OutputFormat string
}

// PolicySettingsOrigin identifies the highest-priority managed settings origin currently represented in the Go host.
type PolicySettingsOrigin string

const (
	// PolicySettingsOriginFile identifies file-based managed settings loaded from managed-settings.json and drop-ins.
	PolicySettingsOriginFile PolicySettingsOrigin = "file"
)

// PolicySettingsConfig stores the minimum managed settings metadata needed by `/status`.
type PolicySettingsConfig struct {
	// Origin identifies which managed settings channel produced the effective policy layer.
	Origin PolicySettingsOrigin
	// HasBaseFile reports whether managed-settings.json contributed non-empty settings.
	HasBaseFile bool
	// HasDropIns reports whether managed-settings.d/*.json files were discovered.
	HasDropIns bool
}

// RemoteSessionConfig stores the minimum runtime context needed by the Go host's remote-mode surfaces.
type RemoteSessionConfig struct {
	// Enabled reports whether bootstrap accepted `--remote` for this process.
	Enabled bool
	// SessionID identifies the synthesized remote session associated with the process.
	SessionID string
	// URL stores the browser-visible remote session URL consumed by `/session`.
	URL string
	// StreamURL stores the optional machine-consumable remote stream endpoint used by runtime subscription wiring.
	StreamURL string
	// InitialPrompt preserves the optional `--remote` description for future runtime wiring.
	InitialPrompt string
}

// PermissionConfig stores the minimal migrated permission settings used for runtime summaries.
type PermissionConfig struct {
	// DefaultMode controls the default approval mode derived from settings when provided.
	DefaultMode string
	// Allow lists explicit allow rules from settings.
	Allow []string
	// Deny lists explicit deny rules from settings.
	Deny []string
	// Ask lists explicit prompt rules from settings.
	Ask []string
	// AdditionalDirectories lists extra directories included in permission scope.
	AdditionalDirectories []string
	// AdditionalDirectoryEntries tracks extra working directories together with their effective source.
	AdditionalDirectoryEntries []AdditionalDirectoryConfig
	// DisableBypassPermissionsMode preserves the literal disable marker when bypass mode is turned off.
	DisableBypassPermissionsMode string
}

// DefaultConfig returns the minimum configuration required by the single-turn text runtime.
func DefaultConfig() Config {
	return Config{
		Model:         "claude-sonnet-4-5",
		EffortLevel:   "",
		Theme:         NormalizeThemeSetting(""),
		EditorMode:    NormalizeEditorMode(""),
		Provider:      ProviderAnthropic,
		ApprovalMode:  "default",
		Env:           map[string]string{},
		RemoteSession: RemoteSessionConfig{},
		Permissions: PermissionConfig{
			DefaultMode: "default",
		},
	}
}

// OAuthAccountConfig stores the minimum cached account metadata needed by `/status`.
type OAuthAccountConfig struct {
	// AccountUUID identifies the cached Claude account when available.
	AccountUUID string
	// EmailAddress stores the cached Claude account email address.
	EmailAddress string
	// OrganizationUUID identifies the cached Claude organization when available.
	OrganizationUUID string
	// OrganizationName stores the cached Claude organization display name.
	OrganizationName string
}

// AdditionalDirectorySource identifies where one effective extra working directory came from.
type AdditionalDirectorySource string

const (
	// AdditionalDirectorySourceUserSettings marks directories loaded from user settings.
	AdditionalDirectorySourceUserSettings AdditionalDirectorySource = "userSettings"
	// AdditionalDirectorySourceProjectSettings marks directories loaded from project settings.
	AdditionalDirectorySourceProjectSettings AdditionalDirectorySource = "projectSettings"
	// AdditionalDirectorySourceLocalSettings marks directories loaded from local settings.
	AdditionalDirectorySourceLocalSettings AdditionalDirectorySource = "localSettings"
	// AdditionalDirectorySourcePolicySettings marks directories loaded from managed policy settings.
	AdditionalDirectorySourcePolicySettings AdditionalDirectorySource = "policySettings"
	// AdditionalDirectorySourceSession marks directories added only for the current process session.
	AdditionalDirectorySourceSession AdditionalDirectorySource = "session"
)

// AdditionalDirectoryConfig stores one effective extra working directory together with its source label.
type AdditionalDirectoryConfig struct {
	// Path stores the stable directory path used by the current runtime snapshot.
	Path string
	// Source records which settings layer or runtime path contributed the directory.
	Source AdditionalDirectorySource
}

// NewAdditionalDirectoryConfigs tags one string directory list with a stable source label.
func NewAdditionalDirectoryConfigs(paths []string, source AdditionalDirectorySource) []AdditionalDirectoryConfig {
	if len(paths) == 0 {
		return nil
	}

	entries := make([]AdditionalDirectoryConfig, 0, len(paths))
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		entries = append(entries, AdditionalDirectoryConfig{
			Path:   trimmed,
			Source: source,
		})
	}
	if len(entries) == 0 {
		return nil
	}
	return entries
}

// AdditionalDirectoryPaths extracts the path list from one sourced directory slice.
func AdditionalDirectoryPaths(entries []AdditionalDirectoryConfig) []string {
	if len(entries) == 0 {
		return nil
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry.Path)
		if trimmed == "" {
			continue
		}
		paths = append(paths, trimmed)
	}
	if len(paths) == 0 {
		return nil
	}
	return paths
}

const remoteSessionBaseURL = "https://claude.ai"

// BuildRemoteSessionURL converts one session id into the minimum claude.ai session URL used by `/session`.
func BuildRemoteSessionURL(sessionID string) string {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return ""
	}
	return fmt.Sprintf("%s/code/%s?m=0", remoteSessionBaseURL, trimmed)
}

const (
	// ThemeSettingAuto identifies the source-compatible auto-follow terminal theme setting.
	ThemeSettingAuto = "auto"
	// ThemeSettingDark identifies the default dark theme setting.
	ThemeSettingDark = "dark"
	// ThemeSettingLight identifies the light theme setting.
	ThemeSettingLight = "light"
	// ThemeSettingLightDaltonized identifies the light colorblind-friendly setting.
	ThemeSettingLightDaltonized = "light-daltonized"
	// ThemeSettingDarkDaltonized identifies the dark colorblind-friendly setting.
	ThemeSettingDarkDaltonized = "dark-daltonized"
	// ThemeSettingLightANSI identifies the light ANSI-only theme setting.
	ThemeSettingLightANSI = "light-ansi"
	// ThemeSettingDarkANSI identifies the dark ANSI-only theme setting.
	ThemeSettingDarkANSI = "dark-ansi"
	// EditorModeNormal identifies the default prompt editor mode used by current settings.
	EditorModeNormal = "normal"
	// EditorModeVim identifies the Vim-style prompt editor mode.
	EditorModeVim = "vim"
	// EditorModeEmacs identifies the legacy source-compatible value that should normalize to normal mode.
	EditorModeEmacs = "emacs"
	// EffortLevelLow identifies the low persisted effort setting.
	EffortLevelLow = "low"
	// EffortLevelMedium identifies the medium persisted effort setting.
	EffortLevelMedium = "medium"
	// EffortLevelHigh identifies the high persisted effort setting.
	EffortLevelHigh = "high"
	// EffortLevelMax identifies the max persisted effort setting.
	EffortLevelMax = "max"
)

var supportedThemeSettings = []string{
	ThemeSettingAuto,
	ThemeSettingDark,
	ThemeSettingLight,
	ThemeSettingLightDaltonized,
	ThemeSettingDarkDaltonized,
	ThemeSettingLightANSI,
	ThemeSettingDarkANSI,
}

var supportedEffortLevels = []string{
	EffortLevelLow,
	EffortLevelMedium,
	EffortLevelHigh,
	EffortLevelMax,
}

// SupportedThemeSettings returns the stable theme-setting values migrated in the Go host.
func SupportedThemeSettings() []string {
	return append([]string(nil), supportedThemeSettings...)
}

// IsSupportedThemeSetting reports whether a string matches one of the migrated theme-setting values.
func IsSupportedThemeSetting(value string) bool {
	switch value {
	case ThemeSettingAuto,
		ThemeSettingDark,
		ThemeSettingLight,
		ThemeSettingLightDaltonized,
		ThemeSettingDarkDaltonized,
		ThemeSettingLightANSI,
		ThemeSettingDarkANSI:
		return true
	default:
		return false
	}
}

// NormalizeThemeSetting folds empty theme values into the stable runtime default.
func NormalizeThemeSetting(value string) string {
	if value == "" {
		return ThemeSettingDark
	}
	return value
}

// NormalizeEditorMode folds legacy and empty editor mode values into the stable runtime representation.
func NormalizeEditorMode(value string) string {
	switch value {
	case "", EditorModeEmacs, EditorModeNormal:
		return EditorModeNormal
	case EditorModeVim:
		return EditorModeVim
	default:
		return value
	}
}

// SupportedEffortLevels returns the stable persisted effort values migrated in the Go host.
func SupportedEffortLevels() []string {
	return append([]string(nil), supportedEffortLevels...)
}

// IsSupportedEffortLevel reports whether a string matches one of the migrated effort-setting values.
func IsSupportedEffortLevel(value string) bool {
	switch value {
	case EffortLevelLow,
		EffortLevelMedium,
		EffortLevelHigh,
		EffortLevelMax:
		return true
	default:
		return false
	}
}

// NormalizeEffortLevel folds empty values into the stable "auto" representation while preserving explicit enum values.
func NormalizeEffortLevel(value string) string {
	switch value {
	case "":
		return ""
	case EffortLevelLow, EffortLevelMedium, EffortLevelHigh, EffortLevelMax:
		return value
	default:
		return value
	}
}
