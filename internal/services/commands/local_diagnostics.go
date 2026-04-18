package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

const maxMemoryDiagnosticBytes = 40000

// LocalDiagnosticsOptions bundles the host signals consumed by `/status` and `/doctor`.
type LocalDiagnosticsOptions struct {
	// Config carries the resolved runtime configuration used for project-scoped diagnostics.
	Config coreconfig.Config
	// ToolRegistry exposes the currently wired tool set so MCP-style tool presence can be summarized.
	ToolRegistry coretool.Registry
	// LookupEnv allows tests to provide stable IDE-related terminal environment signals.
	LookupEnv func(string) (string, bool)
	// Stat allows tests to control filesystem existence checks for memory diagnostics.
	Stat func(string) (os.FileInfo, error)
	// ReadFile allows tests to control memory file contents without touching the host filesystem.
	ReadFile func(string) ([]byte, error)
	// LookPath allows tests to control installation-health probing for the ripgrep binary.
	LookPath func(string) (string, error)
}

// localDiagnosticLines renders the shared local host diagnostics used by `/status` and `/doctor`.
func localDiagnosticLines(opts LocalDiagnosticsOptions) []string {
	return []string{
		fmt.Sprintf("- Bash sandbox: %s", sandboxDiagnosticSummary()),
		fmt.Sprintf("- IDE: %s", ideDiagnosticSummary(opts.ToolRegistry, opts.LookupEnv)),
		fmt.Sprintf("- MCP servers: %s", mcpDiagnosticSummary(opts.ToolRegistry)),
		fmt.Sprintf("- Memory files: %s", memoryDiagnosticSummary(opts)),
		fmt.Sprintf("- Installation health: %s", installationDiagnosticSummary(opts.LookPath)),
	}
}

// sandboxDiagnosticSummary reports the current stable fallback until Bash sandbox wiring exists in the Go host.
func sandboxDiagnosticSummary() string {
	return "not available in Claude Code Go yet"
}

// ideDiagnosticSummary reports whether the current host can see IDE MCP wiring or a likely terminal IDE.
func ideDiagnosticSummary(registry coretool.Registry, lookupEnvFn func(string) (string, bool)) string {
	ideToolCount := 0
	if registry != nil {
		for _, item := range registry.List() {
			if item == nil {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(item.Name()))
			if strings.HasPrefix(name, "mcp__ide__") {
				ideToolCount++
			}
		}
	}
	if ideToolCount > 0 {
		return fmt.Sprintf("connected via IDE MCP (%d tool(s) registered)", ideToolCount)
	}

	lookupEnv := lookupEnvFn
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}

	if ideName := terminalIDEName(lookupEnv); ideName != "" {
		return fmt.Sprintf("terminal session appears to be running inside %s", ideName)
	}

	return "not detected"
}

// mcpDiagnosticSummary reports how many registered tools currently look like MCP proxies.
func mcpDiagnosticSummary(registry coretool.Registry) string {
	if registry == nil {
		return "no MCP tools registered"
	}

	count := 0
	for _, item := range registry.List() {
		if item == nil {
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(item.Name())), "mcp__") {
			count++
		}
	}
	if count == 0 {
		return "no MCP tools registered"
	}
	return fmt.Sprintf("%d MCP tool(s) registered", count)
}

// terminalIDEName infers the likely IDE host from stable terminal environment variables.
func terminalIDEName(lookupEnv func(string) (string, bool)) string {
	if lookupEnv == nil {
		return ""
	}

	jetBrainsName, ok := lookupEnv("JETBRAINS_IDE")
	if ok && strings.TrimSpace(jetBrainsName) != "" {
		return strings.TrimSpace(jetBrainsName)
	}

	terminalEmulator, ok := lookupEnv("TERMINAL_EMULATOR")
	if ok && strings.EqualFold(strings.TrimSpace(terminalEmulator), "JetBrains-JediTerm") {
		return "JetBrains IDE"
	}

	termProgram, ok := lookupEnv("TERM_PROGRAM")
	if ok {
		switch strings.ToLower(strings.TrimSpace(termProgram)) {
		case "vscode":
			if _, hasCursor := lookupEnv("CURSOR_TRACE_ID"); hasCursor {
				return "Cursor"
			}
			return "VS Code"
		case "cursor":
			return "Cursor"
		case "windsurf":
			return "Windsurf"
		case "jetbrains-jediterm":
			return "JetBrains IDE"
		}
	}

	if _, ok := lookupEnv("VSCODE_CWD"); ok {
		return "VS Code"
	}

	return ""
}

// memoryDiagnosticSummary reports whether the current workspace has oversized CLAUDE.md files in the upward search path.
func memoryDiagnosticSummary(opts LocalDiagnosticsOptions) string {
	projectPath := strings.TrimSpace(opts.Config.ProjectPath)
	if projectPath == "" {
		return "project path not configured"
	}

	statFn := opts.Stat
	if statFn == nil {
		statFn = os.Stat
	}
	readFileFn := opts.ReadFile
	if readFileFn == nil {
		readFileFn = os.ReadFile
	}

	candidates := claudeMemoryCandidates(projectPath)
	foundAny := false
	large := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		info, err := statFn(candidate)
		if err != nil || info == nil || info.IsDir() {
			continue
		}
		foundAny = true
		content, err := readFileFn(candidate)
		if err != nil {
			return fmt.Sprintf("unable to read %s", candidate)
		}
		if len(content) > maxMemoryDiagnosticBytes {
			large = append(large, fmt.Sprintf("%s (%d bytes)", candidate, len(content)))
		}
	}

	if len(large) == 1 {
		return fmt.Sprintf("large CLAUDE.md detected: %s > %d bytes", large[0], maxMemoryDiagnosticBytes)
	}
	if len(large) > 1 {
		return fmt.Sprintf("%d large CLAUDE.md files detected (> %d bytes)", len(large), maxMemoryDiagnosticBytes)
	}
	if foundAny {
		return "no large CLAUDE.md files detected"
	}
	return "no CLAUDE.md files detected"
}

// claudeMemoryCandidates enumerates CLAUDE.md candidates from the project path upward to filesystem root.
func claudeMemoryCandidates(projectPath string) []string {
	cleaned := filepath.Clean(projectPath)
	candidates := []string{}
	seen := map[string]struct{}{}
	for {
		candidate := filepath.Join(cleaned, "CLAUDE.md")
		if _, ok := seen[candidate]; !ok {
			candidates = append(candidates, candidate)
			seen[candidate] = struct{}{}
		}
		parent := filepath.Dir(cleaned)
		if parent == cleaned {
			break
		}
		cleaned = parent
	}
	return candidates
}

// installationDiagnosticSummary reports the minimum local installation-health signal currently available in the Go host.
func installationDiagnosticSummary(lookPathFn func(string) (string, error)) string {
	if lookPathFn == nil {
		lookPathFn = exec.LookPath
	}
	path, err := lookPathFn("rg")
	if err != nil {
		return "ripgrep missing from PATH"
	}
	return fmt.Sprintf("ripgrep available at %s", path)
}
