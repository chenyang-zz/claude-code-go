package sandbox

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// SandboxManager manages the lifecycle and configuration of the sandbox system.
// It provides platform detection, dependency checks, and sandbox decision logic.
type SandboxManager struct {
	config     Config
	enabled    bool
	excludeEng *ExcludeEngine
}

// NewSandboxManager creates a new sandbox manager with the given configuration.
func NewSandboxManager(cfg Config) *SandboxManager {
	mgr := &SandboxManager{
		config:     cfg,
		excludeEng: NewExcludeEngine(cfg.ExcludedCommands),
	}
	mgr.enabled = cfg.Enabled && IsSupportedPlatform() && mgr.dependenciesMet()
	return mgr
}

// IsSandboxingEnabled reports whether the sandbox system is active.
func (m *SandboxManager) IsSandboxingEnabled() bool {
	return m.enabled
}

// shouldUseSandbox implements the TS-side shouldUseSandbox decision flow.
// Returns true if the command should run inside the sandbox.
func (m *SandboxManager) shouldUseSandbox(command string, dangerouslyDisableSandbox bool) bool {
	if !m.IsSandboxingEnabled() {
		return false
	}

	// Respect dangerouslyDisableSandbox when allowUnsandboxedCommands is true
	if dangerouslyDisableSandbox && m.config.AllowUnsandboxedCommands {
		return false
	}

	if command == "" {
		return false
	}

	// Check excluded commands
	if m.excludeEng != nil && m.excludeEng.IsExcluded(command) {
		return false
	}

	return true
}

// UpdateConfig replaces the current configuration and refreshes state.
func (m *SandboxManager) UpdateConfig(cfg Config) {
	m.config = cfg
	m.excludeEng = NewExcludeEngine(cfg.ExcludedCommands)
	m.enabled = cfg.Enabled && IsSupportedPlatform() && m.dependenciesMet()
}

// GetStatus returns the current sandbox status for display.
func (m *SandboxManager) GetStatus() SandboxStatus {
	deps := m.CheckDependencies()
	status := SandboxStatus{
		Enabled:           m.config.Enabled,
		Supported:         IsSupportedPlatform(),
		Dependencies:      deps,
		SettingsLockedByPolicy: false, // Policy check deferred to callers
		ExcludedCommands:  m.config.ExcludedCommands,
	}

	platform := DetectPlatform()
	status.PlatformInEnabledList = true // policy-level check deferred

	if !status.Supported {
		switch platform {
		case PlatformWSL1:
			status.UnsupportedPlatformReason = "WSL1 is not supported (requires WSL2)"
		default:
			status.UnsupportedPlatformReason = fmt.Sprintf("Sandboxing is not supported on %s", runtime.GOOS)
		}
	}

	if status.Enabled && status.Supported {
		if len(deps.Errors) > 0 {
			status.UnavailableReason = fmt.Sprintf("dependencies missing: %s", joinStrings(deps.Errors, ", "))
		} else {
			status.Available = true
		}
	}

	return status
}

// CheckDependencies checks whether the required sandbox tools are available.
func (m *SandboxManager) CheckDependencies() DependencyCheck {
	result := DependencyCheck{}

	switch runtime.GOOS {
	case "darwin":
		// macOS uses sandbox-exec (built-in)
		if _, err := exec.LookPath("sandbox-exec"); err != nil {
			result.Errors = append(result.Errors, "sandbox-exec not found (should be built-in on macOS)")
		}
	case "linux":
		// Linux uses bubblewrap + socat for sandboxing
		if _, err := exec.LookPath("bwrap"); err != nil {
			result.Errors = append(result.Errors, "bwrap (bubblewrap) not found")
		}
		if _, err := exec.LookPath("socat"); err != nil {
			result.Warnings = append(result.Warnings, "socat not found (needed for network sandboxing)")
		}
	}

	// Docker (optional, checked separately)
	if _, err := exec.LookPath("docker"); err != nil {
		result.Warnings = append(result.Warnings, "docker not found (Docker sandbox unavailable, falling back to OS-level sandbox)")
	}

	return result
}

// dependenciesMet reports whether the minimum required dependencies exist.
func (m *SandboxManager) dependenciesMet() bool {
	deps := m.CheckDependencies()
	return len(deps.Errors) == 0
}

// joinStrings joins strings with a separator using Builder.
func joinStrings(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(items[0])
	for _, s := range items[1:] {
		b.WriteString(sep)
		b.WriteString(s)
	}
	return b.String()
}

// DockerCheck checks if Docker is available for container-based sandboxing.
func DockerCheck() bool {
	_, err := exec.LookPath("docker")
	if err != nil {
		return false
	}
	// Quick docker ping to verify it's running
	cmd := exec.Command("docker", "info", "--format", "{{.ServerVersion}}")
	return cmd.Run() == nil
}
