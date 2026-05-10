package sandbox

// Config stores the typed sandbox configuration derived from settings.
// Mirrors the TS-side SandboxSettings schema (src/entrypoints/sandboxTypes.ts).
type Config struct {
	// Enabled controls whether sandboxing is active.
	Enabled bool `json:"enabled"`

	// FailIfUnavailable causes a startup error when the sandbox is enabled
	// but cannot start (missing dependencies, unsupported platform).
	FailIfUnavailable bool `json:"failIfUnavailable,omitempty"`

	// AutoAllowBashIfSandboxed automatically allows Bash commands when
	// the sandbox is active.
	AutoAllowBashIfSandboxed bool `json:"autoAllowBashIfSandboxed,omitempty"`

	// AllowUnsandboxedCommands controls whether dangerouslyDisableSandbox
	// is respected. When false, sandbox bypass is completely ignored.
	AllowUnsandboxedCommands bool `json:"allowUnsandboxedCommands,omitempty"`

	// Network holds network isolation configuration.
	Network *NetworkConfig `json:"network,omitempty"`

	// Filesystem holds filesystem restriction configuration.
	Filesystem *FilesystemConfig `json:"filesystem,omitempty"`

	// ExcludedCommands lists command patterns that bypass sandboxing.
	ExcludedCommands []string `json:"excludedCommands,omitempty"`
}

// NetworkConfig mirrors the TS-side SandboxNetworkConfigSchema.
type NetworkConfig struct {
	AllowedDomains      []string `json:"allowedDomains,omitempty"`
	DeniedDomains       []string `json:"deniedDomains,omitempty"`
	AllowUnixSockets    []string `json:"allowUnixSockets,omitempty"`
	AllowAllUnixSockets bool     `json:"allowAllUnixSockets,omitempty"`
	AllowLocalBinding   bool     `json:"allowLocalBinding,omitempty"`
	HTTPProxyPort       int      `json:"httpProxyPort,omitempty"`
	SocksProxyPort      int      `json:"socksProxyPort,omitempty"`
}

// FilesystemConfig mirrors the TS-side SandboxFilesystemConfigSchema.
type FilesystemConfig struct {
	AllowWrite []string `json:"allowWrite,omitempty"`
	DenyWrite  []string `json:"denyWrite,omitempty"`
	DenyRead   []string `json:"denyRead,omitempty"`
	AllowRead  []string `json:"allowRead,omitempty"`
}

// DependencyCheck carries the structured result from checking sandbox
// system dependencies (bubblewrap, socat, sandbox-exec).
type DependencyCheck struct {
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// SandboxStatus carries the aggregate sandbox status for the /sandbox command.
type SandboxStatus struct {
	Enabled                   bool            `json:"enabled"`
	Supported                 bool            `json:"supported"`
	Dependencies              DependencyCheck `json:"dependencies"`
	PlatformInEnabledList     bool            `json:"platformInEnabledList"`
	SettingsLockedByPolicy    bool            `json:"settingsLockedByPolicy"`
	ExcludedCommands          []string        `json:"excludedCommands"`
	Available                 bool            `json:"available"`
	UnavailableReason         string          `json:"unavailableReason,omitempty"`
	UnsupportedPlatformReason string          `json:"unsupportedPlatformReason,omitempty"`
}
