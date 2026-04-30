package plugin

import (
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
func toLspServerConfig(s *LspServerConfig) lsp.ServerConfig {
	return lsp.ServerConfig{
		Command: s.Command,
		Args:    s.Args,
		Env:     s.Env,
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
