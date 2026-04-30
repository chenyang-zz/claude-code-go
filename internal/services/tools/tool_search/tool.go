// Package tool_search implements the ToolSearch tool that lets models discover
// deferred tools by name or description.
package tool_search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the tool identifier registered in the tool set.
	Name = "ToolSearch"
	// toolDescription provides a short summary for the tool.
	toolDescription = "Search for deferred tools by name or description. Use this to discover tools that are not currently loaded. Use \"select:<tool_name>\" to load a specific tool, or keywords to search."
)

// SharedRegistry is populated by bootstrap after the tool registry is created.
// ToolSearchTool needs access to the registry to list all registered tools.
var SharedRegistry coretool.Registry

// Input defines the expected arguments for the ToolSearch tool.
type Input struct {
	// Query contains the search query. Use "select:<tool_name>" for direct
	// selection, or keywords to search.
	Query string `json:"query"`
	// MaxResults limits the number of returned matches (default 5).
	MaxResults int `json:"max_results,omitempty"`
}

// Output contains the search result.
type Output struct {
	// Matches lists the names of tools matching the search query.
	Matches []string `json:"matches"`
	// Query echoes back the original search query.
	Query string `json:"query"`
	// TotalDeferredTools reports the total number of deferred tools available.
	TotalDeferredTools int `json:"total_deferred_tools"`
}

// Tool implements the coretool.Tool interface for deferred tool discovery.
type Tool struct{}

// NewTool returns a new ToolSearch tool instance.
func NewTool() *Tool {
	return &Tool{}
}

// Name returns the stable tool identifier.
func (t *Tool) Name() string {
	return Name
}

// Description returns a short human-readable summary.
func (t *Tool) Description() string {
	return toolDescription
}

// InputSchema declares the expected query and max_results fields.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"query": {
				Type:        coretool.ValueKindString,
				Description: "Query to find deferred tools. Use \"select:<tool_name>\" for direct selection, or keywords to search.",
				Required:    true,
			},
			"max_results": {
				Type:        coretool.ValueKindInteger,
				Description: "Maximum number of results to return (default: 5).",
				Required:    false,
			},
		},
	}
}

// IsReadOnly reports that this tool does not mutate external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that multiple invocations can run in parallel.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke executes the tool search.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("tool_search: nil receiver")
	}

	if SharedRegistry == nil {
		return coretool.Result{Error: "tool_search: tool registry not initialized"}, nil
	}

	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	query := strings.TrimSpace(input.Query)
	if query == "" {
		return coretool.Result{Error: "tool_search: query must not be empty"}, nil
	}

	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}

	tools := SharedRegistry.List()
	var matches []string

	// Direct selection via "select:<name>" prefix.
	// Supports comma-separated multi-select: select:A,B,C.
	if selectMatch := strings.TrimPrefix(query, "select:"); selectMatch != query {
		names := strings.Split(selectMatch, ",")
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			for _, t := range tools {
				if strings.EqualFold(t.Name(), name) {
					matches = append(matches, t.Name())
					break
				}
			}
		}

		logger.DebugCF("tool_search", "select match", map[string]any{
			"query":   query,
			"matches": matches,
		})
	} else {
		// Keyword search: case-insensitive substring match on tool name and description.
		queryLower := strings.ToLower(query)
		for _, t := range tools {
			nameLower := strings.ToLower(t.Name())
			descLower := strings.ToLower(t.Description())
			if strings.Contains(nameLower, queryLower) || strings.Contains(descLower, queryLower) {
				matches = append(matches, t.Name())
				if len(matches) >= maxResults {
					break
				}
			}
		}

		logger.DebugCF("tool_search", "keyword search", map[string]any{
			"query":   query,
			"matches": len(matches),
		})
	}

	output := Output{
		Matches:             matches,
		Query:               query,
		TotalDeferredTools:  len(tools),
	}

	// Ensure matches is never nil in JSON output.
	if output.Matches == nil {
		output.Matches = []string{}
	}

	return coretool.Result{
		Output: formatOutputJSON(output),
		Meta:   map[string]any{"data": output},
	}, nil
}

// formatOutputJSON marshals the output struct into a JSON string.
func formatOutputJSON(output Output) string {
	data, err := json.Marshal(output)
	if err != nil {
		return `{"matches":[],"query":"","total_deferred_tools":0}`
	}
	return string(data)
}
