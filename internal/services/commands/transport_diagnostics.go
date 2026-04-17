package commands

import (
	"fmt"
	"strings"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// transportDiagnosticLines renders the migrated proxy and TLS diagnostics shared by /status and /doctor.
func transportDiagnosticLines(cfg coreconfig.Config) []string {
	lines := make([]string, 0, 4)
	if value := strings.TrimSpace(cfg.ProxyURL); value != "" {
		lines = append(lines, fmt.Sprintf("- Proxy: %s", value))
	}
	if value := strings.TrimSpace(cfg.AdditionalCACertsPath); value != "" {
		lines = append(lines, fmt.Sprintf("- Additional CA cert(s): %s", value))
	}
	if value := strings.TrimSpace(cfg.MTLSClientCertPath); value != "" {
		lines = append(lines, fmt.Sprintf("- mTLS client cert: %s", value))
	}
	if value := strings.TrimSpace(cfg.MTLSClientKeyPath); value != "" {
		lines = append(lines, fmt.Sprintf("- mTLS client key: %s", value))
	}
	return lines
}
