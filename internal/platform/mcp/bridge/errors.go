package bridge

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// defaultMcpToolTimeout is the default timeout for MCP tool invocations.
// It matches the TS-side getMcpToolTimeoutMs() behavior of providing a
// bounded execution window.  The value is 60 seconds rather than the
// TS-side 27.8 h default because the Go host currently only supports
// stdio transport where hung servers are more common.
const defaultMcpToolTimeout = 60 * time.Second

// envMcpToolTimeout is the environment variable that overrides the default
// MCP tool timeout.  Value is parsed as milliseconds to stay consistent
// with the TS-side MCP_TOOL_TIMEOUT variable.
const envMcpToolTimeout = "MCP_TOOL_TIMEOUT"

// getMcpToolTimeout returns the timeout to use for a single MCP tool call.
// It respects the MCP_TOOL_TIMEOUT environment variable (milliseconds) and
// falls back to defaultMcpToolTimeout.
func getMcpToolTimeout() time.Duration {
	raw := os.Getenv(envMcpToolTimeout)
	if raw == "" {
		return defaultMcpToolTimeout
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms <= 0 {
		return defaultMcpToolTimeout
	}
	return time.Duration(ms) * time.Millisecond
}

// McpAuthError indicates that an MCP tool call failed because of an
// authentication problem (e.g. expired OAuth token returning 401).
type McpAuthError struct {
	ServerName string
	Message    string
}

// Error implements the error interface.
func (e *McpAuthError) Error() string {
	return fmt.Sprintf("mcp auth error [%s]: %s", e.ServerName, e.Message)
}

// IsAuthError always returns true for McpAuthError.
func (e *McpAuthError) IsAuthError() bool { return true }

// IsSessionExpired always returns false for McpAuthError.
func (e *McpAuthError) IsSessionExpired() bool { return false }

// McpSessionExpiredError indicates that the MCP session expired during the
// tool call (e.g. HTTP 404 or connection closed mid-request).
type McpSessionExpiredError struct {
	ServerName string
}

// Error implements the error interface.
func (e *McpSessionExpiredError) Error() string {
	return fmt.Sprintf("mcp session expired [%s]", e.ServerName)
}

// IsAuthError always returns false for McpSessionExpiredError.
func (e *McpSessionExpiredError) IsAuthError() bool { return false }

// IsSessionExpired always returns true for McpSessionExpiredError.
func (e *McpSessionExpiredError) IsSessionExpired() bool { return true }

// McpToolCallError indicates that the MCP server accepted the request but
// the tool itself returned an error (isError=true in the result).
type McpToolCallError struct {
	ServerName string
	ToolName   string
	Message    string
	Meta       map[string]any
}

// Error implements the error interface.
func (e *McpToolCallError) Error() string {
	return fmt.Sprintf("mcp tool call error [%s/%s]: %s", e.ServerName, e.ToolName, e.Message)
}

// IsAuthError always returns false for McpToolCallError.
func (e *McpToolCallError) IsAuthError() bool { return false }

// IsSessionExpired always returns false for McpToolCallError.
func (e *McpToolCallError) IsSessionExpired() bool { return false }

// classifyMcpError inspects a raw error returned by the MCP client and
// attempts to classify it into one of the structured error types above.
// When no specific pattern matches the error is returned unchanged.
func classifyMcpError(serverName, toolName string, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	lower := strings.ToLower(msg)

	// Auth errors: 401, Unauthorized, or token expiry indicators.
	if strings.Contains(lower, "401") ||
		strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "token expired") ||
		strings.Contains(lower, "authentication") {
		return &McpAuthError{
			ServerName: serverName,
			Message:    msg,
		}
	}

	// Session expiry: connection closed, 404, or session-specific keywords.
	if strings.Contains(lower, "connection closed") ||
		strings.Contains(lower, "404") ||
		strings.Contains(lower, "session expired") ||
		strings.Contains(lower, "session not found") {
		return &McpSessionExpiredError{
			ServerName: serverName,
		}
	}

	return err
}
