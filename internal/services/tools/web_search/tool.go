package web_search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the provider-facing tool name registered in the tool catalog.
	Name = "WebSearch"
	// toolDescription is shown to the model during tool selection.
	toolDescription = "Search the web for current information. Returns search results with titles and URLs formatted as markdown hyperlinks."
	// maxSearchUses caps the number of web searches the model may execute internally.
	maxSearchUses = 8
)

// Input represents the validated input for the WebSearch tool.
type Input struct {
	Query          string   `json:"query"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	BlockedDomains []string `json:"blocked_domains,omitempty"`
}

// Output represents the structured output returned by the WebSearch tool.
type Output struct {
	Query           string  `json:"query"`
	Results         string  `json:"results"`
	DurationSeconds float64 `json:"durationSeconds"`
}

// Tool implements the WebSearch tool, allowing models to search the web
// via the Anthropic web_search_20250305 server tool.
type Tool struct {
	client model.Client
	model  string
}

// NewTool creates a WebSearch tool wired to the given model client.
// modelName is the model used for the sub-call (typically the main loop model).
func NewTool(client model.Client, modelName string) *Tool {
	return &Tool{client: client, model: modelName}
}

// Name returns the provider-facing tool name.
func (t *Tool) Name() string { return Name }

// Description returns the tool description shown to the model.
func (t *Tool) Description() string { return toolDescription }

// InputSchema describes the expected JSON input shape.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"query": {
				Type:        coretool.ValueKindString,
				Description: "The search query to use (minimum 2 characters)",
				Required:    true,
			},
			"allowed_domains": {
				Type:        coretool.ValueKindArray,
				Description: "Only include search results from these domains",
				Required:    false,
			},
			"blocked_domains": {
				Type:        coretool.ValueKindArray,
				Description: "Never include search results from these domains",
				Required:    false,
			},
		},
	}
}

// IsReadOnly reports that web search does not modify any files.
func (t *Tool) IsReadOnly() bool { return true }

// IsConcurrencySafe reports that multiple web searches may run concurrently.
func (t *Tool) IsConcurrencySafe() bool { return true }

// Invoke executes the web search by making a sub-model call with the
// web_search_20250305 server tool and collecting the results.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("web_search: nil receiver")
	}
	if t.client == nil {
		return coretool.Result{Error: "web_search: model client not initialized"}, nil
	}

	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	query := strings.TrimSpace(input.Query)
	if len(query) < 2 {
		return coretool.Result{Error: "web_search: query must be at least 2 characters"}, nil
	}

	startTime := time.Now()

	// Build the web_search_20250305 server tool schema.
	searchTool := map[string]any{
		"type":     "web_search_20250305",
		"name":     "web_search",
		"max_uses": float64(maxSearchUses),
	}
	if len(input.AllowedDomains) > 0 {
		allowed := make([]any, 0, len(input.AllowedDomains))
		for _, d := range input.AllowedDomains {
			allowed = append(allowed, d)
		}
		searchTool["allowed_domains"] = allowed
	}
	if len(input.BlockedDomains) > 0 {
		blocked := make([]any, 0, len(input.BlockedDomains))
		for _, d := range input.BlockedDomains {
			blocked = append(blocked, d)
		}
		searchTool["blocked_domains"] = blocked
	}

	// Build the sub-call request.
	req := model.Request{
		Model: t.model,
		System: "You are an assistant for performing a web search tool use",
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.TextPart("Perform a web search for the query: " + query),
				},
			},
		},
		ExtraToolSchemas: []map[string]any{searchTool},
	}

	logger.DebugCF("web_search", "starting web search sub-call", map[string]any{
		"query": query,
		"model": req.Model,
	})

	stream, err := t.client.Stream(ctx, req)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("web_search: model call failed: %v", err)}, nil
	}

	// Collect all content from the stream.
	var textParts []string
	var searchHits []model.WebSearchHit
	hasResults := false

	for evt := range stream {
		switch evt.Type {
		case model.EventTypeTextDelta:
			textParts = append(textParts, evt.Text)
		case model.EventTypeServerToolUse:
			// Track server-side tool usage.
			logger.DebugCF("web_search", "server tool use", map[string]any{
				"tool_id":   evt.ServerToolUse.ID,
				"tool_name": evt.ServerToolUse.Name,
			})
		case model.EventTypeWebSearchResult:
			if evt.WebSearchResult.ErrorCode != "" {
				textParts = append(textParts, fmt.Sprintf("Web search error: %s", evt.WebSearchResult.ErrorCode))
				continue
			}
			hasResults = true
			for _, hit := range evt.WebSearchResult.Content {
				searchHits = append(searchHits, hit)
			}
		case model.EventTypeError:
			return coretool.Result{Error: fmt.Sprintf("web_search: stream error: %s", evt.Error)}, nil
		}
	}

	durationSeconds := time.Since(startTime).Seconds()

	// Format the output.
	var formatted strings.Builder
	fmt.Fprintf(&formatted, "Web search results for query: %q\n\n", query)

	if len(textParts) > 0 {
		formatted.WriteString(strings.Join(textParts, ""))
		formatted.WriteString("\n\n")
	}

	if hasResults {
		if len(searchHits) > 0 {
			formatted.WriteString("Links: ")
			hitJSON, _ := json.Marshal(searchHits)
			formatted.WriteString(string(hitJSON))
			formatted.WriteString("\n\n")
		} else {
			formatted.WriteString("No links found.\n\n")
		}
	}

	formatted.WriteString("\nREMINDER: You MUST include the sources above in your response to the user using markdown hyperlinks.")

	output := Output{
		Query:           query,
		Results:         strings.TrimSpace(formatted.String()),
		DurationSeconds: durationSeconds,
	}

	logger.DebugCF("web_search", "search complete", map[string]any{
		"query":      query,
		"hit_count":  len(searchHits),
		"duration_s": durationSeconds,
	})

	return coretool.Result{
		Output: output.Results,
		Meta:   map[string]any{"data": output},
	}, nil
}

