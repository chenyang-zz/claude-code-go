package config

import (
	"fmt"
	"strings"
)

// Config stores the minimal runtime configuration currently consumed by the Go host.
type Config struct {
	// ProjectPath identifies the current workspace path used for project-scoped runtime behavior.
	ProjectPath string
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
}

// RemoteSessionConfig stores the minimum runtime context needed by the Go host's remote-mode surfaces.
type RemoteSessionConfig struct {
	// Enabled reports whether bootstrap accepted `--remote` for this process.
	Enabled bool
	// SessionID identifies the synthesized remote session associated with the process.
	SessionID string
	// URL stores the browser-visible remote session URL consumed by `/session`.
	URL string
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
