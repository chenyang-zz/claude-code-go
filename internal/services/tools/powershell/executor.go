package powershell

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	platformshell "github.com/sheepzhao/claude-code-go/internal/platform/shell"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// psExecutables lists the available PowerShell executables in priority order.
var psExecutables = []string{"pwsh", "pwsh.exe", "powershell", "powershell.exe"}

// detectPowerShellPath finds the first available PowerShell executable.
// On Windows, powershell.exe is always available. On non-Windows, only pwsh
// (PowerShell Core 7+) is available if installed.
func detectPowerShellPath() (string, []string) {
	for _, exe := range psExecutables {
		if path, err := exec.LookPath(exe); err == nil {
			switch strings.ToLower(exe) {
			case "pwsh", "pwsh.exe":
				return path, []string{"-NoProfile", "-NonInteractive", "-Command"}
			case "powershell", "powershell.exe":
				return path, []string{"-NoProfile", "-Command"}
			}
		}
	}
	return "", nil
}

// NewExecutor creates a PowerShell executor that uses pwsh (preferred) or
// powershell.exe (Windows fallback). Returns nil when no PowerShell is available.
func NewExecutor() *platformshell.Executor {
	psPath, psArgs := detectPowerShellPath()
	if psPath == "" {
		logger.WarnCF("powershell_executor", "no PowerShell executable found", map[string]any{
			"os": runtime.GOOS,
		})
		return nil
	}

	return &platformshell.Executor{
		ShellLookup: func() (string, []string) {
			return psPath, psArgs
		},
		Environ: os.Environ,
	}
}

// IsPowerShellAvailable returns true when a usable PowerShell executable
// has been detected on the current system.
func IsPowerShellAvailable() bool {
	path, _ := exec.LookPath("pwsh")
	if path != "" {
		return true
	}
	if runtime.GOOS == "windows" {
		path, _ = exec.LookPath("powershell.exe")
		return path != ""
	}
	return false
}

// normalizePSCommand normalizes a PowerShell command string for permission
// matching. It lowercases the first token (cmdlet name) and resolves common
// aliases to canonical cmdlet names.
func normalizePSCommand(command string) string {
	tokens := strings.Fields(strings.TrimSpace(command))
	if len(tokens) == 0 {
		return ""
	}

	// Lowercase and resolve alias for the first token
	first := strings.ToLower(tokens[0])
	if canonical, ok := psCommonAliases[first]; ok {
		tokens[0] = canonical
	} else {
		tokens[0] = first
	}

	return strings.Join(tokens, " ")
}

// psCommonAliases maps PowerShell aliases (lowercase) to canonical cmdlet names (lowercase).
// Ported from TS src/utils/powershell/parser.ts COMMON_ALIASES.
var psCommonAliases = map[string]string{
	// Directory listing
	"ls":   "get-childitem",
	"dir":  "get-childitem",
	"gci":  "get-childitem",
	// Content
	"cat":  "get-content",
	"type": "get-content",
	"gc":   "get-content",
	// Navigation
	"cd":    "set-location",
	"sl":    "set-location",
	"chdir": "set-location",
	"pushd": "push-location",
	"popd":  "pop-location",
	"pwd":   "get-location",
	"gl":    "get-location",
	// Items
	"gi":     "get-item",
	"gp":     "get-itemproperty",
	"ni":     "new-item",
	"mkdir":  "new-item",
	"md":     "new-item",
	"ri":     "remove-item",
	"del":    "remove-item",
	"rd":     "remove-item",
	"rmdir":  "remove-item",
	"rm":     "remove-item",
	"erase":  "remove-item",
	"mi":     "move-item",
	"mv":     "move-item",
	"move":   "move-item",
	"ci":     "copy-item",
	"cp":     "copy-item",
	"copy":   "copy-item",
	"cpi":    "copy-item",
	"si":     "set-item",
	"rni":    "rename-item",
	"ren":    "rename-item",
	// Process
	"ps":    "get-process",
	"gps":   "get-process",
	"kill":  "stop-process",
	"spps":  "stop-process",
	"start": "start-process",
	"saps":  "start-process",
	"sajb":  "start-job",
	"ipmo":  "import-module",
	// Output
	"echo":  "write-output",
	"write": "write-output",
	"sleep": "start-sleep",
	// Help
	"help": "get-help",
	"man":  "get-help",
	"gcm":  "get-command",
	// Service
	"gsv": "get-service",
	// Variables
	"gv": "get-variable",
	"sv": "set-variable",
	// History
	"h":       "get-history",
	"history": "get-history",
	// Invoke
	"iex": "invoke-expression",
	"iwr": "invoke-webrequest",
	"irm": "invoke-restmethod",
	"icm": "invoke-command",
	"ii":  "invoke-item",
	// PSSession
	"nsn":  "new-pssession",
	"etsn": "enter-pssession",
	"exsn": "exit-pssession",
	"gsn":  "get-pssession",
	"rsn":  "remove-pssession",
	// Misc
	"cls":      "clear-host",
	"clear":    "clear-host",
	"select":   "select-object",
	"where":    "where-object",
	"foreach":  "foreach-object",
	"%":        "foreach-object",
	"?":        "where-object",
	"measure":  "measure-object",
	"ft":       "format-table",
	"fl":       "format-list",
	"fw":       "format-wide",
	"oh":       "out-host",
	"ogv":      "out-gridview",
	"ac":       "add-content",
	"clc":      "clear-content",
	"tee":      "tee-object",
	"epcsv":    "export-csv",
	"sp":       "set-itemproperty",
	"rp":       "remove-itemproperty",
	"cli":      "clear-item",
	"epal":     "export-alias",
	"sls":      "select-string",
}

// formatDuration formats a timeout in milliseconds as a Go duration string.
func formatDuration(ms int) string {
	if ms <= 0 {
		return "2m0s"
	}
	return fmt.Sprintf("%dms", ms)
}

// resolvePSCommand resolves an alias to its canonical name.
// Returns the lowercase canonical name, or the lowercase input if not an alias.
func resolvePSCommand(name string) string {
	lower := strings.ToLower(name)
	if canonical, ok := psCommonAliases[lower]; ok {
		return canonical
	}
	return lower
}

// isPSDangerousCmdlet returns true when the command name matches a known
// dangerous PowerShell cmdlet that can execute arbitrary code.
func isPSDangerousCmdlet(name string) bool {
	canonical := resolvePSCommand(name)

	dangerous := map[string]bool{
		"invoke-expression":      true,
		"iex":                    true,
		"invoke-command":         true,
		"invoke-webrequest":      true,
		"invoke-restmethod":      true,
		"start-job":              true,
		"start-threadjob":        true,
		"register-scheduledjob":  true,
		"add-type":               true,
		"new-object":             true,
		"start-process":          true,
		"import-module":          true,
		"install-module":         true,
		"save-module":            true,
		"update-module":          true,
		"install-script":         true,
		"save-script":            true,
		"invoke-wmimethod":       true,
		"invoke-cimmethod":       true,
		"invoke-item":            true,
		"register-scheduledtask": true,
		"new-scheduledtask":      true,
		"new-scheduledtaskaction": true,
		"set-scheduledtask":      true,
	}

	return dangerous[canonical]
}
