package mcp

import (
	"context"
	"strings"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	mcpclient "github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
)

// TestAuthToolName verifies the pseudo auth tool name is stable.
func TestAuthToolName(t *testing.T) {
	tool := NewAuthTool("proxy", mcpclient.ServerConfig{})
	if got := tool.Name(); got != "proxy__authenticate" {
		t.Fatalf("Name() = %q, want proxy__authenticate", got)
	}
}

// TestAuthToolInvokeReportsManualGuidance verifies the pseudo auth tool returns guidance and structured metadata.
func TestAuthToolInvokeReportsManualGuidance(t *testing.T) {
	tool := NewAuthTool("proxy", mcpclient.ServerConfig{
		Type: "http",
		OAuth: &mcpclient.OAuthConfig{
			ClientID:              "client-123",
			AuthServerMetadataURL: "https://auth.example.invalid/.well-known/oauth-authorization-server",
		},
	})

	result, err := tool.Invoke(context.Background(), coretool.Call{})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Output, "proxy") {
		t.Fatalf("Invoke() output = %q, want server name guidance", result.Output)
	}

	data, ok := result.Meta["data"].(authToolOutput)
	if !ok {
		t.Fatalf("Invoke() meta data type = %T, want authToolOutput", result.Meta["data"])
	}
	if data.Status != "manual" {
		t.Fatalf("Invoke() status = %q, want manual", data.Status)
	}
	if !data.OAuthConfigured {
		t.Fatal("Invoke() OAuthConfigured = false, want true")
	}
	if data.AuthServerMetadataURL == "" {
		t.Fatal("Invoke() AuthServerMetadataURL = empty, want auth metadata url")
	}
}

// TestAuthToolInvokeReportsClaudeAIConnector verifies claude.ai connectors get the connector-specific guidance.
func TestAuthToolInvokeReportsClaudeAIConnector(t *testing.T) {
	tool := NewAuthTool("claudeai", mcpclient.ServerConfig{Type: "claudeai-proxy"})

	result, err := tool.Invoke(context.Background(), coretool.Call{})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Output, "/mcp") {
		t.Fatalf("Invoke() output = %q, want /mcp guidance", result.Output)
	}
}
