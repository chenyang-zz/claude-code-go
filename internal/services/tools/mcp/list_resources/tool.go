// Package list_resources provides the ListMcpResourcesTool which enumerates
// resources exposed by connected MCP servers.
package list_resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registration name for the ListMcpResources tool.
	Name = "ListMcpResources"
)

// Tool enumerates resources exposed by connected MCP servers.
type Tool struct{}

// NewTool creates a new ListMcpResourcesTool instance.
// The tool accesses the shared MCP registry at runtime via GetLastRegistry.
func NewTool() *Tool {
	return &Tool{}
}

// Name returns the stable registration identifier.
func (t *Tool) Name() string {
	return Name
}

// Description returns a short human-readable summary.
func (t *Tool) Description() string {
	return `Lists available resources from configured MCP servers. Each resource object includes a 'server' field indicating which server it's from. Use without arguments to list all resources from all servers, or pass an optional 'server' parameter to filter by server name.`
}

// InputSchema declares the expected input shape.
func (t *Tool) InputSchema() tool.InputSchema {
	return tool.InputSchema{
		Properties: map[string]tool.FieldSchema{
			"server": {
				Type:        tool.ValueKindString,
				Description: "Optional server name to filter resources by.",
				Required:    false,
			},
		},
	}
}

// IsReadOnly reports that listing resources does not mutate external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that multiple concurrent invocations are safe.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// listResourcesInput is the decoded input payload for the tool.
type listResourcesInput struct {
	Server string `json:"server,omitempty"`
}

// resourceEntry mirrors the TS output shape with an added server field.
type resourceEntry struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	MimeType    string `json:"mimeType,omitempty"`
	Description string `json:"description,omitempty"`
	Server      string `json:"server"`
}

// Invoke executes the list resources operation.
func (t *Tool) Invoke(ctx context.Context, call tool.Call) (tool.Result, error) {
	_ = ctx

	// Decode input.
	input, err := tool.DecodeInput[listResourcesInput](t.InputSchema(), call.Input)
	if err != nil {
		return tool.Result{Error: fmt.Sprintf("invalid input: %v", err)}, nil
	}

	registry := mcpregistry.GetLastRegistry()
	if registry == nil {
		return tool.Result{Output: "[]"}, nil
	}

	var entries []mcpregistry.Entry
	if input.Server != "" {
		entry, found := registry.GetEntry(input.Server)
		if !found {
			return tool.Result{Error: fmt.Sprintf("Server %q not found.", input.Server)}, nil
		}
		entries = []mcpregistry.Entry{entry}
	} else {
		entries = registry.List()
	}

	// Collect resources from connected entries that support resources.
	var resources []resourceEntry
	for _, entry := range entries {
		if entry.Status != mcpregistry.StatusConnected {
			continue
		}
		if entry.Capabilities.Resources == nil {
			continue
		}
		for _, r := range entry.Resources {
			resources = append(resources, resourceEntry{
				URI:         r.URI,
				Name:        r.Name,
				MimeType:    r.MimeType,
				Description: r.Description,
				Server:      entry.Name,
			})
		}
	}

	if resources == nil {
		resources = []resourceEntry{}
	}

	output, err := json.Marshal(resources)
	if err != nil {
		logger.WarnCF("list_resources", "failed to marshal resource list", map[string]any{
			"error": err.Error(),
		})
		return tool.Result{Error: "Failed to serialize resource list."}, nil
	}

	logger.DebugCF("list_resources", "listed MCP resources", map[string]any{
		"count":   len(resources),
		"servers": len(entries),
	})

	return tool.Result{Output: string(output)}, nil
}

// Ensure Tool satisfies the tool.Tool interface.
var _ tool.Tool = (*Tool)(nil)
