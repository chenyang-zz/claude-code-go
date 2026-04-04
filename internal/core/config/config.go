package config

// Config stores the minimal runtime configuration currently consumed by the Go host.
type Config struct {
	// Model overrides the default model selection when provided.
	Model string
	// Provider selects which backend provider implementation to use.
	Provider string
	// ApprovalMode controls the runtime approval behavior.
	ApprovalMode string
	// SessionDBPath points at the session persistence database when enabled.
	SessionDBPath string
}
