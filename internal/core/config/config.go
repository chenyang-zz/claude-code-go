package config

// Config stores the minimal runtime configuration currently consumed by the Go host.
type Config struct {
	// ProjectPath identifies the current workspace path used for project-scoped runtime behavior.
	ProjectPath string
	// Model overrides the default model selection when provided.
	Model string
	// Provider selects which backend provider implementation to use.
	Provider string
	// APIKey carries the credential required by the selected provider.
	APIKey string
	// APIBaseURL optionally overrides the provider API endpoint.
	APIBaseURL string
	// ApprovalMode controls the runtime approval behavior.
	ApprovalMode string
	// SessionDBPath points at the session persistence database when enabled.
	SessionDBPath string
	// Permissions stores the migrated read-only permission settings surfaced to slash commands.
	Permissions PermissionConfig
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
		Model:        "claude-sonnet-4-5",
		Provider:     "anthropic",
		ApprovalMode: "default",
		Permissions: PermissionConfig{
			DefaultMode: "default",
		},
	}
}
