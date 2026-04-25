package bridge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// AdaptTool converts an MCP tool declaration into a core tool.Tool wrapper.
func AdaptTool(serverName string, mcpTool client.Tool, mcpClient *client.Client) tool.Tool {
	return &ProxyTool{
		serverName: serverName,
		mcpTool:    mcpTool,
		client:     mcpClient,
	}
}

// ProxyTool implements core.Tool by delegating invocations to an MCP server.
type ProxyTool struct {
	serverName string
	mcpTool    client.Tool
	client     *client.Client
}

// Name returns the fully-qualified tool name ("serverName__toolName").
func (p *ProxyTool) Name() string {
	return fmt.Sprintf("%s__%s", p.serverName, p.mcpTool.Name)
}

// Description returns the MCP tool description.
func (p *ProxyTool) Description() string {
	return p.mcpTool.Description
}

// InputSchema converts the MCP JSON Schema into the minimal core schema.
func (p *ProxyTool) InputSchema() tool.InputSchema {
	return convertInputSchema(p.mcpTool.InputSchema)
}

// IsReadOnly reports the readOnly hint from MCP annotations.
func (p *ProxyTool) IsReadOnly() bool {
	if p.mcpTool.Annotations == nil {
		return false
	}
	return p.mcpTool.Annotations.ReadOnlyHint
}

// IsConcurrencySafe mirrors IsReadOnly for MCP tools.
func (p *ProxyTool) IsConcurrencySafe() bool {
	return p.IsReadOnly()
}

// Invoke calls the MCP server and returns the tool result.
func (p *ProxyTool) Invoke(ctx context.Context, call tool.Call) (tool.Result, error) {
	logger.DebugCF("mcp", "invoking proxy tool", map[string]any{
		"server":  p.serverName,
		"tool":    p.mcpTool.Name,
		"call_id": call.ID,
	})

	start := time.Now()

	// Report start progress when a callback is available.
	tool.ReportProgress(ctx, map[string]any{
		"type":       "mcp_progress",
		"status":     "started",
		"serverName": p.serverName,
		"toolName":   p.mcpTool.Name,
	})

	// Apply a bounded timeout so a hung MCP server does not block the engine loop.
	timeout := getMcpToolTimeout()
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req := client.CallToolRequest{
		Name:      p.mcpTool.Name,
		Arguments: call.Input,
	}

	result, err := p.client.CallTool(callCtx, req)

	elapsed := time.Since(start)

	// Report finish progress (always, even on error).
	tool.ReportProgress(ctx, map[string]any{
		"type":       "mcp_progress",
		"status":     "finished",
		"serverName": p.serverName,
		"toolName":   p.mcpTool.Name,
		"elapsedMs":  elapsed.Milliseconds(),
	})

	if err != nil {
		classified := classifyMcpError(p.serverName, p.mcpTool.Name, err)
		return tool.Result{
			Error: classified.Error(),
			Meta: map[string]any{
				"server": p.serverName,
				"tool":   p.mcpTool.Name,
				"error":  classified,
			},
		}, nil
	}

	if result.IsError {
		var msg string
		if len(result.Content) > 0 {
			msg = contentToString(result.Content)
		} else {
			msg = "MCP tool returned an error"
		}
		toolErr := &McpToolCallError{
			ServerName: p.serverName,
			ToolName:   p.mcpTool.Name,
			Message:    msg,
			Meta:       result.Meta,
		}
		return tool.Result{
			Error: toolErr.Error(),
			Meta: map[string]any{
				"server": p.serverName,
				"tool":   p.mcpTool.Name,
				"error":  toolErr,
			},
		}, nil
	}

	output := contentToString(result.Content)
	return tool.Result{
		Output: output,
		Meta: map[string]any{
			"server": p.serverName,
			"tool":   p.mcpTool.Name,
		},
	}, nil
}

// contentToString flattens MCP content items into a single string.
func contentToString(items []client.ContentItem) string {
	var parts []string
	for _, item := range items {
		switch item.Type {
		case "text":
			parts = append(parts, item.Text)
		case "image":
			parts = append(parts, fmt.Sprintf("[image: %s]", item.MimeType))
		case "resource":
			parts = append(parts, "[resource]")
		default:
			parts = append(parts, fmt.Sprintf("[%s]", item.Type))
		}
	}
	return strings.Join(parts, "\n")
}

// convertInputSchema maps an MCP JSON Schema into the core tool InputSchema.
func convertInputSchema(schema client.ToolInputSchema) tool.InputSchema {
	props := make(map[string]tool.FieldSchema, len(schema.Properties))
	reqSet := make(map[string]bool, len(schema.Required))
	for _, r := range schema.Required {
		reqSet[r] = true
	}

	for name, raw := range schema.Properties {
		field := tool.FieldSchema{}
		if m, ok := raw.(map[string]any); ok {
			if t, ok := m["type"].(string); ok {
				field.Type = jsonTypeToValueKind(t)
			}
			if d, ok := m["description"].(string); ok {
				field.Description = d
			}
			if items, ok := m["items"].(map[string]any); ok {
				if itemType, ok := items["type"].(string); ok {
					field.Items = &tool.FieldSchema{
						Type: jsonTypeToValueKind(itemType),
					}
				}
			}
		}
		field.Required = reqSet[name]
		props[name] = field
	}

	return tool.InputSchema{Properties: props}
}

// jsonTypeToValueKind maps JSON Schema type strings to core ValueKind values.
func jsonTypeToValueKind(t string) tool.ValueKind {
	switch t {
	case "string":
		return tool.ValueKindString
	case "integer":
		return tool.ValueKindInteger
	case "number":
		return tool.ValueKindNumber
	case "boolean":
		return tool.ValueKindBoolean
	case "object":
		return tool.ValueKindObject
	case "array":
		return tool.ValueKindArray
	default:
		return tool.ValueKindString
	}
}
