package plugin

import (
	"path/filepath"

	"github.com/sheepzhao/claude-code-go/internal/platform/lsp"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// LspRegistrar converts plugin LspServerConfig values into core lsp.ServerConfig
// values and registers them with an LSP Manager.
type LspRegistrar struct {
	manager *lsp.Manager
}

// NewLspRegistrar creates an LSP registrar backed by the given manager.
// If manager is nil, registration calls become no-ops (useful when the LSP
// subsystem has not been initialized yet).
func NewLspRegistrar(manager *lsp.Manager) *LspRegistrar {
	return &LspRegistrar{manager: manager}
}

// RegisterLspServers converts and registers every LSP server configuration
// from the given slice.  If a server with the same name is already
// registered, the new registration is skipped and recorded as an error.
// Advanced fields not supported by the Go LSP manager
// (InitializationOptions, Settings, WorkspaceFolder, StartupTimeout) are
// ignored, matching the boundary decision in analysis-m1 §3.4.
func (r *LspRegistrar) RegisterLspServers(servers []*LspServerConfig) (registered int, errs []*PluginError) {
	if r == nil || r.manager == nil {
		logger.DebugCF("plugin.lsp_registrar", "LSP manager not available, skipping plugin LSP registration", nil)
		return 0, nil
	}

	for _, s := range servers {
		if s == nil {
			continue
		}

		cfg := toLspServerConfig(s)
		extensions := extractExtensions(s.ExtensionToLanguage)

		if err := r.manager.RegisterServer(s.Name, cfg, extensions); err != nil {
			errs = append(errs, &PluginError{
				Type:    "lsp-registration-error",
				Source:  "lsp-registrar",
				Plugin:  s.Name,
				Message: err.Error(),
			})
			logger.WarnCF("plugin.lsp_registrar", "failed to register LSP server", map[string]any{
				"name":  s.Name,
				"error": err.Error(),
			})
			continue
		}

		registered++
		logger.InfoCF("plugin.lsp_registrar", "registered LSP server", map[string]any{
			"name":       s.Name,
			"command":    s.Command,
			"extensions": extensions,
		})
	}

	return registered, errs
}

// toLspServerConfig maps a plugin LspServerConfig to the core lsp.ServerConfig.
// Plugin variables in Command, Args, and Env are substituted, and
// CLAUDE_PLUGIN_ROOT / CLAUDE_PLUGIN_DATA are injected into the environment.
func toLspServerConfig(s *LspServerConfig) lsp.ServerConfig {
	command := s.Command
	args := make([]string, len(s.Args))
	copy(args, s.Args)
	env := make(map[string]string, len(s.Env))
	for k, v := range s.Env {
		env[k] = v
	}

	// Substitute plugin variables when plugin context is available.
	if s.PluginPath != "" || s.PluginSource != "" {
		command = SubstitutePluginVariables(command, s.PluginPath, s.PluginSource)
		for i, arg := range args {
			args[i] = SubstitutePluginVariables(arg, s.PluginPath, s.PluginSource)
		}
		for k, v := range env {
			env[k] = SubstitutePluginVariables(v, s.PluginPath, s.PluginSource)
		}
	}

	// Inject CLAUDE_PLUGIN_ROOT and CLAUDE_PLUGIN_DATA into env.
	if s.PluginPath != "" {
		if _, ok := env["CLAUDE_PLUGIN_ROOT"]; !ok {
			env["CLAUDE_PLUGIN_ROOT"] = filepath.ToSlash(s.PluginPath)
		}
	}
	if s.PluginSource != "" {
		if dataDir, err := GetPluginDataDir(s.PluginSource); err == nil {
			if _, ok := env["CLAUDE_PLUGIN_DATA"]; !ok {
				env["CLAUDE_PLUGIN_DATA"] = filepath.ToSlash(dataDir)
			}
		}
	}

	return lsp.ServerConfig{
		Command: command,
		Args:    args,
		Env:     env,
	}
}

// extractExtensions returns the keys of the ExtensionToLanguage map as a
// slice of extension strings.  These are the file extensions that the
// registered LSP server will handle.
func extractExtensions(extMap map[string]string) []string {
	if len(extMap) == 0 {
		return nil
	}
	extensions := make([]string, 0, len(extMap))
	for ext := range extMap {
		extensions = append(extensions, ext)
	}
	return extensions
}
