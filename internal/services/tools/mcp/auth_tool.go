package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	mcpclient "github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// AuthTool exposes a minimal authenticate pseudo-tool for MCP servers that need auth.
type AuthTool struct {
	serverName string
	config     mcpclient.ServerConfig
}

// NewAuthTool constructs a pseudo-tool that explains how the host should authenticate one MCP server.
func NewAuthTool(serverName string, config mcpclient.ServerConfig) tool.Tool {
	return &AuthTool{
		serverName: strings.TrimSpace(serverName),
		config:     config,
	}
}

// authToolInput is the empty input contract for the pseudo auth tool.
type authToolInput struct{}

// authToolOutput stores the structured auth guidance returned by the pseudo tool.
type authToolOutput struct {
	Status                string `json:"status"`
	Message               string `json:"message"`
	ServerName            string `json:"serverName,omitempty"`
	Transport             string `json:"transport,omitempty"`
	OAuthConfigured       bool   `json:"oauthConfigured,omitempty"`
	AuthServerMetadataURL string `json:"authServerMetadataUrl,omitempty"`
	CallbackPort          *int   `json:"callbackPort,omitempty"`
	XAA                   *bool  `json:"xaa,omitempty"`
}

// Name returns the stable registration name for the pseudo auth tool.
func (t *AuthTool) Name() string {
	if t == nil {
		return "mcp__authenticate"
	}
	return fmt.Sprintf("%s__authenticate", t.serverName)
}

// Description returns the summary exposed to provider tool schemas.
func (t *AuthTool) Description() string {
	return "Use this tool to inspect the authentication state for an MCP server."
}

// InputSchema returns the empty input contract for the pseudo auth tool.
func (t *AuthTool) InputSchema() tool.InputSchema {
	return tool.InputSchema{}
}

// IsReadOnly reports that the pseudo auth tool does not mutate host state.
func (t *AuthTool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that the pseudo auth tool can be invoked in parallel.
func (t *AuthTool) IsConcurrencySafe() bool {
	return true
}

// Invoke renders a stable auth guidance message for one MCP server.
func (t *AuthTool) Invoke(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t == nil {
		return tool.Result{Error: "mcp auth tool: nil receiver"}, nil
	}
	if strings.TrimSpace(t.serverName) == "" {
		return tool.Result{Error: "mcp auth tool: missing server name"}, nil
	}
	if _, err := tool.DecodeInput[authToolInput](t.InputSchema(), call.Input); err != nil {
		return tool.Result{Error: err.Error()}, nil
	}

	var callbackPort *int
	var xaa *bool
	output := authToolOutput{
		Status:       "manual",
		ServerName:   t.serverName,
		Transport:    t.config.Type,
		Message:      t.manualAuthMessage(),
		CallbackPort: callbackPort,
		XAA:          xaa,
	}
	if t.config.OAuth != nil {
		output.OAuthConfigured = true
		output.AuthServerMetadataURL = t.config.OAuth.AuthServerMetadataURL
		output.CallbackPort = t.config.OAuth.CallbackPort
		output.XAA = t.config.OAuth.XAA
	}

	logger.DebugCF("mcp", "rendered auth pseudo-tool guidance", map[string]any{
		"server":           t.serverName,
		"transport":        t.config.Type,
		"oauth_configured": output.OAuthConfigured,
		"status":           output.Status,
	})

	return tool.Result{
		Output: output.Message,
		Meta:   map[string]any{"data": output},
	}, nil
}

// manualAuthMessage builds the stable human-readable guidance for one server.
func (t *AuthTool) manualAuthMessage() string {
	transport := strings.TrimSpace(t.config.Type)
	switch transport {
	case "claudeai-proxy":
		return fmt.Sprintf("The %q MCP server is a claude.ai connector and needs authentication. Use /mcp to authenticate it manually.", t.serverName)
	case "http", "sse":
		if t.config.OAuth == nil {
			return fmt.Sprintf("The %q MCP server needs authentication, but no OAuth metadata is configured yet.", t.serverName)
		}
		if t.config.OAuth.XAA != nil && *t.config.OAuth.XAA {
			return fmt.Sprintf("The %q MCP server is configured for XAA, but Claude Code Go does not launch the browser auth flow yet. Authenticate it manually from /mcp.", t.serverName)
		}
		return fmt.Sprintf("The %q MCP server needs authentication. Claude Code Go exposes this pseudo-tool for visibility, but browser OAuth is not implemented yet. Use /mcp to continue manually.", t.serverName)
	default:
		if transport == "" {
			transport = "stdio"
		}
		return fmt.Sprintf("The %q MCP server uses %s transport and needs authentication, but this transport does not support the pseudo auth tool flow yet. Use /mcp to authenticate it manually.", t.serverName, transport)
	}
}
