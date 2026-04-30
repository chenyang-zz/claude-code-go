// Package read_resource provides the ReadMcpResourceTool which reads a single
// resource from a connected MCP server by URI.
package read_resource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	mcpclient "github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registration name for the ReadMcpResource tool.
	Name = "ReadMcpResource"
)

// Tool reads a specific resource from a connected MCP server.
type Tool struct{}

// NewTool creates a new ReadMcpResourceTool instance.
func NewTool() *Tool {
	return &Tool{}
}

// Name returns the stable registration identifier.
func (t *Tool) Name() string {
	return Name
}

// Description returns a short human-readable summary.
func (t *Tool) Description() string {
	return `Reads a specific resource from an MCP server. Requires the server name and the resource URI. Both 'server' and 'uri' parameters are required.`
}

// InputSchema declares the expected input shape.
func (t *Tool) InputSchema() tool.InputSchema {
	return tool.InputSchema{
		Properties: map[string]tool.FieldSchema{
			"server": {
				Type:        tool.ValueKindString,
				Description: "The MCP server name.",
				Required:    true,
			},
			"uri": {
				Type:        tool.ValueKindString,
				Description: "The resource URI to read.",
				Required:    true,
			},
		},
	}
}

// IsReadOnly reports that reading a resource does not mutate external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that multiple concurrent invocations are safe.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// readResourceInput is the decoded input payload.
type readResourceInput struct {
	Server string `json:"server"`
	URI    string `json:"uri"`
}

// contentEntry mirrors the TS output content entry.
type contentEntry struct {
	URI         string `json:"uri"`
	MimeType    string `json:"mimeType,omitempty"`
	Text        string `json:"text,omitempty"`
	BlobSavedTo string `json:"blobSavedTo,omitempty"`
}

// readResourceOutput wraps the contents array.
type readResourceOutput struct {
	Contents []contentEntry `json:"contents"`
}

// Invoke executes the read resource operation.
func (t *Tool) Invoke(ctx context.Context, call tool.Call) (tool.Result, error) {
	input, err := tool.DecodeInput[readResourceInput](t.InputSchema(), call.Input)
	if err != nil {
		return tool.Result{Error: fmt.Sprintf("invalid input: %v", err)}, nil
	}

	registry := mcpregistry.GetLastRegistry()
	if registry == nil {
		return tool.Result{Error: "No MCP servers are configured."}, nil
	}

	entry, found := registry.GetEntry(input.Server)
	if !found {
		return tool.Result{Error: fmt.Sprintf("Server %q not found.", input.Server)}, nil
	}

	if entry.Status != mcpregistry.StatusConnected {
		return tool.Result{Error: fmt.Sprintf("Server %q is not connected (status: %s).", input.Server, entry.Status)}, nil
	}

	if entry.Capabilities.Resources == nil {
		return tool.Result{Error: fmt.Sprintf("Server %q does not support resources.", input.Server)}, nil
	}

	if entry.Client == nil {
		return tool.Result{Error: fmt.Sprintf("Server %q has no active client.", input.Server)}, nil
	}

	result, err := entry.Client.ReadResource(ctx, mcpclient.ReadResourceRequest{URI: input.URI})
	if err != nil {
		logger.WarnCF("read_resource", "failed to read MCP resource", map[string]any{
			"server": input.Server,
			"uri":    input.URI,
			"error":  err.Error(),
		})
		return tool.Result{Error: fmt.Sprintf("Failed to read resource: %v", err)}, nil
	}

	// Process each content item from the response.
	contents := make([]contentEntry, 0, len(result.Contents))
	for _, raw := range result.Contents {
		content := contentEntry{}
		if uri, ok := raw["uri"].(string); ok {
			content.URI = uri
		}
		if mime, ok := raw["mimeType"].(string); ok {
			content.MimeType = mime
		}
		if text, ok := raw["text"].(string); ok {
			content.Text = text
		}
		// For blob content, indicate binary data availability without persisting to disk.
		if _, hasBlob := raw["blob"]; hasBlob {
			content.Text = "[Binary blob content — persistence not implemented]"
		}
		contents = append(contents, content)
	}

	output := readResourceOutput{Contents: contents}
	outputBytes, err := json.Marshal(output)
	if err != nil {
		logger.WarnCF("read_resource", "failed to marshal read result", map[string]any{
			"error": err.Error(),
		})
		return tool.Result{Error: "Failed to serialize resource content."}, nil
	}

	logger.DebugCF("read_resource", "read MCP resource", map[string]any{
		"server":      input.Server,
		"uri":         input.URI,
		"content_len": len(contents),
	})

	return tool.Result{Output: string(outputBytes)}, nil
}

// Ensure Tool satisfies the tool.Tool interface.
var _ tool.Tool = (*Tool)(nil)
