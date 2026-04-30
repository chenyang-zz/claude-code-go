package lsp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// HandlerRegistrationResult tracks the outcome of registering diagnostic
// notification handlers across multiple LSP servers.
type HandlerRegistrationResult struct {
	TotalServers        int
	SuccessCount        int
	RegistrationErrors  []ServerRegistrationError
	DiagnosticFailures  map[string]*ServerFailureTracker
}

// ServerRegistrationError records a failure to register a handler on a server.
type ServerRegistrationError struct {
	ServerName string
	Error      string
}

// ServerFailureTracker tracks consecutive diagnostic processing failures
// for a specific LSP server.
type ServerFailureTracker struct {
	Count     int
	LastError string
}

// formatDiagnosticsForAttachment converts a raw LSP PublishDiagnosticsParams
// notification into the internal DiagnosticFile format. It normalizes file://
// URIs to file system paths.
func formatDiagnosticsForAttachment(params PublishDiagnosticsParams) []DiagnosticFile {
	uri := params.URI
	if strings.HasPrefix(uri, "file://") {
		if parsed, err := url.Parse(uri); err == nil {
			uri = parsed.Path
		}
	}

	diagnostics := make([]Diagnostic, 0, len(params.Diagnostics))
	for _, d := range params.Diagnostics {
		code := ""
		if d.Code != "" {
			code = d.Code
		}
		diagnostics = append(diagnostics, Diagnostic{
			Message:  d.Message,
			Severity: d.Severity,
			Range: Range{
				Start: Position{
					Line:      d.Range.Start.Line,
					Character: d.Range.Start.Character,
				},
				End: Position{
					Line:      d.Range.End.Line,
					Character: d.Range.End.Character,
				},
			},
			Source: d.Source,
			Code:   code,
		})
	}

	return []DiagnosticFile{
		{
			URI:         uri,
			Diagnostics: diagnostics,
		},
	}
}

// RegisterDiagnosticHandlers registers a textDocument/publishDiagnostics
// notification handler on every running LSP server managed by the given
// Manager, and on servers that start later. Diagnostics are routed into
// the provided registry for async delivery.
//
// Returns tracking data for registration status and runtime failures.
func RegisterDiagnosticHandlers(manager *Manager, registry *LSPDiagnosticRegistry) *HandlerRegistrationResult {
	servers := manager.GetAllServers()

	result := &HandlerRegistrationResult{
		TotalServers:       len(servers),
		DiagnosticFailures: make(map[string]*ServerFailureTracker),
	}

	for serverName := range servers {
		err := registerOnServer(manager, serverName, registry, result)
		if err != nil {
			result.RegistrationErrors = append(result.RegistrationErrors, ServerRegistrationError{
				ServerName: serverName,
				Error:      err.Error(),
			})
			logger.DebugCF("lsp.diagnostics", "failed to register handler for server", map[string]any{
				"server": serverName,
				"error":  err.Error(),
			})
			continue
		}
		result.SuccessCount++
		logger.DebugCF("lsp.diagnostics", "registered diagnostics handler for server", map[string]any{
			"server": serverName,
		})
	}

	if len(result.RegistrationErrors) > 0 {
		var failedNames []string
		for _, e := range result.RegistrationErrors {
			failedNames = append(failedNames, fmt.Sprintf("%s (%s)", e.ServerName, e.Error))
		}
		logger.DebugCF("lsp.diagnostics", "handler registration summary", map[string]any{
			"success":    result.SuccessCount,
			"total":      result.TotalServers,
			"failed":     strings.Join(failedNames, ", "),
		})
	} else {
		logger.DebugCF("lsp.diagnostics", "handlers registered for all servers", map[string]any{
			"total": result.TotalServers,
		})
	}

	return result
}

// registerOnServer sets up the publishDiagnostics handler for a single server.
func registerOnServer(manager *Manager, serverName string, registry *LSPDiagnosticRegistry, result *HandlerRegistrationResult) error {
	handler := func(rawParams json.RawMessage) {
		tracker := result.DiagnosticFailures[serverName]
		if tracker == nil {
			tracker = &ServerFailureTracker{}
			result.DiagnosticFailures[serverName] = tracker
		}

		logger.DebugCF("lsp.diagnostics", "handler invoked", map[string]any{
			"server": serverName,
		})

		var params PublishDiagnosticsParams
		if err := json.Unmarshal(rawParams, &params); err != nil {
			tracker.Count++
			tracker.LastError = fmt.Sprintf("unmarshal error: %v", err)
			logger.DebugCF("lsp.diagnostics", "invalid diagnostic params", map[string]any{
				"server": serverName,
				"error":  err.Error(),
			})
			if tracker.Count >= 3 {
				logger.DebugCF("lsp.diagnostics", "consecutive failures warning", map[string]any{
					"server": serverName,
					"count":  tracker.Count,
					"lastError": tracker.LastError,
				})
			}
			return
		}

		if params.URI == "" {
			tracker.Count++
			tracker.LastError = "missing uri in diagnostic params"
			logger.DebugCF("lsp.diagnostics", "missing uri in diagnostic params", map[string]any{
				"server": serverName,
			})
			if tracker.Count >= 3 {
				logger.DebugCF("lsp.diagnostics", "consecutive failures warning", map[string]any{
					"server": serverName,
					"count":  tracker.Count,
					"lastError": tracker.LastError,
				})
			}
			return
		}

		logger.DebugCF("lsp.diagnostics", "received diagnostics from server", map[string]any{
			"server":     serverName,
			"count":      len(params.Diagnostics),
			"uri":        params.URI,
		})

		diagnosticFiles := formatDiagnosticsForAttachment(params)
		firstFile := diagnosticFiles[0]
		if len(diagnosticFiles) == 0 || len(firstFile.Diagnostics) == 0 {
			logger.DebugCF("lsp.diagnostics", "skipping empty diagnostics", map[string]any{
				"server": serverName,
				"uri":    params.URI,
			})
			return
		}

		registry.RegisterPending(serverName, diagnosticFiles)
		logger.DebugCF("lsp.diagnostics", "registered diagnostics for async delivery", map[string]any{
			"server":          serverName,
			"fileCount":       len(diagnosticFiles),
			"diagnosticCount": len(firstFile.Diagnostics),
		})

		// Reset failure counter on success.
		delete(result.DiagnosticFailures, serverName)
	}

	return manager.OnNotification(serverName, "textDocument/publishDiagnostics", handler)
}
