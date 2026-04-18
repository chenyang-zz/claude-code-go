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

// DoctorCommand renders a minimum host diagnostic summary for the current Go CLI runtime.
type DoctorCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
	// ToolRegistry exposes the currently wired tool set for shared local host diagnostics.
	ToolRegistry coretool.Registry
	// Stat inspects local filesystem paths so tests can supply stable results.
	Stat func(string) (os.FileInfo, error)
	// ReadFile inspects memory files for shared local diagnostics.
	ReadFile func(string) ([]byte, error)
	// LookPath inspects host binaries for shared installation-health diagnostics.
	LookPath func(string) (string, error)
	// LookupEnv inspects terminal environment signals for shared IDE diagnostics.
	LookupEnv func(string) (string, bool)
}

// Metadata returns the canonical slash descriptor for /doctor.
func (c DoctorCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "doctor",
		Description: "Diagnose the current Claude Code Go host setup",
		Usage:       "/doctor",
	}
}

// Execute summarizes the minimum local runtime diagnostics that can be checked without remote calls.
func (c DoctorCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	lines := []string{
		"Doctor summary:",
		fmt.Sprintf("- Provider: %s", displayValue(c.Config.Provider)),
		fmt.Sprintf("- Model: %s", displayValue(c.Config.Model)),
		fmt.Sprintf("- API key: %s", secretStatus(c.Config.APIKey)),
		fmt.Sprintf("- API base URL: %s", baseURLValue(c.Config.APIBaseURL)),
		fmt.Sprintf("- Project path: %s", projectPathDiagnosis(c.Config.ProjectPath)),
		fmt.Sprintf("- Approval mode: %s", displayValue(c.Config.ApprovalMode)),
		fmt.Sprintf("- Session DB: %s", c.sessionDBDiagnosis()),
	}
	lines = append(lines, transportDiagnosticLines(c.Config)...)
	lines = append(lines, localDiagnosticLines(LocalDiagnosticsOptions{
		Config:       c.Config,
		ToolRegistry: c.ToolRegistry,
		LookupEnv:    c.LookupEnv,
		Stat:         c.Stat,
		ReadFile:     c.ReadFile,
		LookPath:     c.LookPath,
	})...)

	logger.DebugCF("commands", "rendered doctor command output", map[string]any{
		"provider":            c.Config.Provider,
		"model":               c.Config.Model,
		"project_path":        c.Config.ProjectPath,
		"approval_mode":       c.Config.ApprovalMode,
		"has_api_key":         c.Config.APIKey != "",
		"has_session_db_path": c.Config.SessionDBPath != "",
		"has_proxy":           c.Config.ProxyURL != "",
		"has_extra_ca":        c.Config.AdditionalCACertsPath != "",
		"has_mtls_cert":       c.Config.MTLSClientCertPath != "",
		"has_mtls_key":        c.Config.MTLSClientKeyPath != "",
	})

	return command.Result{
		Output: strings.Join(lines, "\n"),
	}, nil
}

// sessionDBDiagnosis describes whether the configured session database path is locally usable.
func (c DoctorCommand) sessionDBDiagnosis() string {
	path := strings.TrimSpace(c.Config.SessionDBPath)
	if path == "" {
		return "missing"
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

// projectPathDiagnosis renders one stable diagnosis line for the current project scope.
func projectPathDiagnosis(path string) string {
	if strings.TrimSpace(path) == "" {
		return "missing"
	}
	return path
}
