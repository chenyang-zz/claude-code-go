package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// APIConnectivityProbeResult stores the caller-facing outcome of one provider connectivity probe.
type APIConnectivityProbeResult struct {
	// Summary is the rendered line body inserted into the /status output.
	Summary string
}

// APIConnectivityProber defines the minimum provider-side connectivity probe used by /status.
type APIConnectivityProber interface {
	// Probe checks whether the configured provider endpoint is reachable enough for a status summary.
	Probe(ctx context.Context, cfg coreconfig.Config) APIConnectivityProbeResult
}

// StatusCommand renders a minimum host status summary for the current Go CLI runtime.
type StatusCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
	// ToolRegistry exposes the currently wired tool set for host status summaries.
	ToolRegistry coretool.Registry
	// APIProbe performs provider-specific connectivity checks when available.
	APIProbe APIConnectivityProber
	// Stat inspects local filesystem paths so tests can provide stable storage diagnostics.
	Stat func(string) (os.FileInfo, error)
	// ReadFile inspects memory files for shared local diagnostics.
	ReadFile func(string) ([]byte, error)
	// LookPath inspects host binaries for shared installation-health diagnostics.
	LookPath func(string) (string, error)
}

// Metadata returns the canonical slash descriptor for /status.
func (c StatusCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "status",
		Description: "Show Claude Code status including version, model, account, API connectivity, and tool statuses",
		Usage:       "/status",
	}
}

// Execute summarizes the stable local status signals that are currently available in the Go host.
func (c StatusCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = args

	apiProbe := c.apiConnectivityStatus(ctx)
	toolSummary, toolCount := statusToolSummary(c.ToolRegistry)
	lines := []string{
		"Status summary:",
		fmt.Sprintf("- Provider: %s", displayValue(c.Config.Provider)),
		fmt.Sprintf("- API provider type: %s", statusProviderType(c.Config.Provider)),
		fmt.Sprintf("- Model: %s", displayValue(c.Config.Model)),
		fmt.Sprintf("- Project path: %s", displayValue(c.Config.ProjectPath)),
		fmt.Sprintf("- Approval mode: %s", displayValue(c.Config.ApprovalMode)),
		fmt.Sprintf("- Session storage: %s", c.sessionStorageStatus()),
		fmt.Sprintf("- Settings sources: %s", statusSettingSources(c.Config.LoadedSettingSources)),
		fmt.Sprintf("- Account auth: %s", statusAccountAuth(c.Config)),
		fmt.Sprintf("- API key source: %s", statusCredentialSource(c.Config.APIKeySource)),
		fmt.Sprintf("- Auth token source: %s", statusCredentialSource(c.Config.AuthTokenSource)),
		fmt.Sprintf("- API base URL: %s", baseURLValue(c.Config.APIBaseURL)),
		fmt.Sprintf("- API base URL source: %s", statusBaseURLSource(c.Config)),
	}
	lines = append(lines, transportDiagnosticLines(c.Config)...)
	lines = append(lines, localDiagnosticLines(LocalDiagnosticsOptions{
		Config:       c.Config,
		ToolRegistry: c.ToolRegistry,
		Stat:         c.Stat,
		ReadFile:     c.ReadFile,
		LookPath:     c.LookPath,
	})...)
	lines = append(lines,
		fmt.Sprintf("- API connectivity check: %s", apiProbe.Summary),
		fmt.Sprintf("- Tool status checks: %s", toolSummary),
		"- Settings status UI: not available in Claude Code Go yet",
	)

	logger.DebugCF("commands", "rendered status command output", map[string]any{
		"provider":            c.Config.Provider,
		"model":               c.Config.Model,
		"project_path":        c.Config.ProjectPath,
		"approval_mode":       c.Config.ApprovalMode,
		"has_api_key":         c.Config.APIKey != "",
		"has_auth_token":      c.Config.AuthToken != "",
		"has_session_db_path": c.Config.SessionDBPath != "",
		"has_proxy":           c.Config.ProxyURL != "",
		"has_extra_ca":        c.Config.AdditionalCACertsPath != "",
		"has_mtls_cert":       c.Config.MTLSClientCertPath != "",
		"has_mtls_key":        c.Config.MTLSClientKeyPath != "",
		"settings_sources":    strings.Join(c.Config.LoadedSettingSources, ","),
		"api_connectivity":    apiProbe.Summary,
		"tool_count":          toolCount,
	})

	return command.Result{
		Output: strings.Join(lines, "\n"),
	}, nil
}

// sessionStorageStatus reports whether the configured session persistence path is locally usable.
func (c StatusCommand) sessionStorageStatus() string {
	path := strings.TrimSpace(c.Config.SessionDBPath)
	if path == "" {
		return "not configured"
	}

	statFn := c.Stat
	if statFn == nil {
		statFn = os.Stat
	}

	if _, err := statFn(path); err == nil {
		return fmt.Sprintf("%s (present)", path)
	}

	parent := filepath.Dir(path)
	if _, err := statFn(parent); err == nil {
		return fmt.Sprintf("%s (not created yet; parent directory exists)", path)
	}

	return fmt.Sprintf("%s (parent directory missing)", path)
}

// apiConnectivityStatus renders the provider-specific connectivity outcome or a stable fallback.
func (c StatusCommand) apiConnectivityStatus(ctx context.Context) APIConnectivityProbeResult {
	if missingStatusCredential(c.Config) {
		return APIConnectivityProbeResult{
			Summary: "skipped (missing auth credential)",
		}
	}
	if c.APIProbe == nil {
		return APIConnectivityProbeResult{
			Summary: fmt.Sprintf("not supported for provider %s", displayValue(c.Config.Provider)),
		}
	}
	return c.APIProbe.Probe(ctx, c.Config)
}

// statusToolSummary reports the currently wired tool registry in one stable text line.
func statusToolSummary(registry coretool.Registry) (string, int) {
	if registry == nil {
		return "no tools registered", 0
	}

	registered := registry.List()
	names := make([]string, 0, len(registered))
	for _, item := range registered {
		if item == nil {
			continue
		}
		names = append(names, item.Name())
	}
	if len(names) == 0 {
		return "no tools registered", 0
	}
	return fmt.Sprintf("%d registered (%s)", len(names), strings.Join(names, ", ")), len(names)
}

// statusSessionStorage reports whether session persistence is configured without probing the filesystem.
func statusSessionStorage(path string) string {
	if strings.TrimSpace(path) == "" {
		return "not configured"
	}
	return fmt.Sprintf("configured (%s)", path)
}

// statusAccountAuth reports the stable authentication state currently visible to the Go host.
func statusAccountAuth(cfg coreconfig.Config) string {
	if strings.TrimSpace(cfg.APIKey) != "" {
		return "API key configured; interactive account status is not available"
	}
	if strings.TrimSpace(cfg.AuthToken) != "" {
		return "Auth token configured; interactive account status is not available"
	}
	return "missing auth credential; interactive account status is not available"
}

// statusProviderType renders the stable provider-family label used by `/status`.
func statusProviderType(provider string) string {
	switch coreconfig.NormalizeProvider(provider) {
	case coreconfig.ProviderAnthropic:
		return "Anthropic first-party"
	case coreconfig.ProviderOpenAICompatible:
		return "OpenAI-compatible"
	case coreconfig.ProviderGLM:
		return "GLM-compatible"
	default:
		return displayValue(provider)
	}
}

// statusSettingSources renders the loaded settings sources in a stable human-readable order.
func statusSettingSources(sources []string) string {
	if len(sources) == 0 {
		return "none"
	}

	labels := make([]string, 0, len(sources))
	for _, source := range sources {
		switch strings.TrimSpace(source) {
		case "userSettings":
			labels = append(labels, "User settings")
		case "projectSettings":
			labels = append(labels, "Project settings")
		case "localSettings":
			labels = append(labels, "Local settings")
		case "flagSettings":
			labels = append(labels, "--settings")
		default:
			labels = append(labels, source)
		}
	}
	return strings.Join(labels, ", ")
}

// statusCredentialSource renders the tracked environment key that supplied one runtime credential.
func statusCredentialSource(source string) string {
	if strings.TrimSpace(source) == "" {
		return "not configured"
	}
	return source
}

// statusBaseURLSource renders either the tracked base URL source or a stable default marker.
func statusBaseURLSource(cfg coreconfig.Config) string {
	if strings.TrimSpace(cfg.APIBaseURL) == "" {
		return "default"
	}
	if strings.TrimSpace(cfg.APIBaseURLSource) == "" {
		return "configured"
	}
	return cfg.APIBaseURLSource
}

// missingStatusCredential reports whether the current provider lacks the minimum credential needed for /status probing.
func missingStatusCredential(cfg coreconfig.Config) bool {
	if coreconfig.NormalizeProvider(cfg.Provider) == coreconfig.ProviderAnthropic {
		return strings.TrimSpace(cfg.APIKey) == "" && strings.TrimSpace(cfg.AuthToken) == ""
	}
	return strings.TrimSpace(cfg.APIKey) == ""
}
