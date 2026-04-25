package web_fetch

import (
	"context"
	"fmt"
	"time"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier used by the migrated WebFetch tool.
	Name = "WebFetch"
	// maxResultSizeChars mirrors the TS tool result persistence threshold.
	maxResultSizeChars = 100_000
)

// Tool implements the minimum migrated WebFetch tool path.
type Tool struct {
	// fetcher performs the actual HTTP GET and markdown conversion.
	fetcher *Fetcher
	// permissions checks WebFetch allow/deny/ask rules before execution starts.
	permissions *PermissionChecker
}

// Input stores the typed request payload accepted by the migrated WebFetch tool.
type Input struct {
	// URL is the fully-formed URL to fetch content from.
	URL string `json:"url"`
	// Prompt describes what information to extract from the fetched content.
	Prompt string `json:"prompt"`
}

// Output stores the structured result metadata returned by the migrated WebFetch tool.
type Output struct {
	// Bytes is the size of the fetched content in bytes.
	Bytes int `json:"bytes"`
	// Code is the HTTP response status code.
	Code int `json:"code"`
	// CodeText is the HTTP response status text.
	CodeText string `json:"codeText"`
	// Result is the processed content (truncated markdown or redirect message).
	Result string `json:"result"`
	// DurationMs is the wall-clock time taken to fetch and process.
	DurationMs int64 `json:"durationMs"`
	// URL is the URL that was fetched.
	URL string `json:"url"`
}

// NewTool constructs a WebFetch tool with the provided cache and permission config.
func NewTool(cache *Cache, allow, deny, ask []string) *Tool {
	return &Tool{
		fetcher:     NewFetcher(cache),
		permissions: NewPermissionChecker(allow, deny, ask),
	}
}

// Name returns the stable registration name for the migrated WebFetch tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return "Fetch content from a URL and return it as markdown."
}

// InputSchema returns the WebFetch tool input contract exposed to model providers.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that WebFetch never mutates external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that independent WebFetch invocations may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke validates input, enforces WebFetch permissions, fetches the URL, and normalizes the result.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("web_fetch tool: nil receiver")
	}
	if t.fetcher == nil {
		return coretool.Result{}, fmt.Errorf("web_fetch tool: fetcher is not configured")
	}
	if t.permissions == nil {
		return coretool.Result{}, fmt.Errorf("web_fetch tool: permission checker is not configured")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	urlStr := input.URL
	if urlStr == "" {
		return coretool.Result{Error: "url is required"}, nil
	}

	// Validate URL format and safety.
	if !validateURL(urlStr) {
		return coretool.Result{Error: fmt.Sprintf("Error: Invalid URL %q. The URL provided could not be parsed.", urlStr)}, nil
	}

	// Permission check.
	evaluation := t.permissions.Check(urlStr)
	if evaluation.Decision == corepermission.DecisionAllow {
		logger.DebugCF("web_fetch_tool", "web fetch allowed", map[string]any{
			"url": urlStr,
		})
	} else if corepermission.HasWebFetchGrant(ctx, call.Name, urlStr, call.Context.WorkingDir) {
		logger.DebugCF("web_fetch_tool", "web fetch allowed by runtime grant", map[string]any{
			"url": urlStr,
		})
	} else if evaluation.Decision == corepermission.DecisionDeny {
		return coretool.Result{
			Error: evaluation.Message,
			Meta: map[string]any{
				"permission_decision": string(evaluation.Decision),
			},
		}, nil
	} else {
		return coretool.Result{}, &corepermission.WebFetchPermissionError{
			ToolName:   call.Name,
			URL:        urlStr,
			WorkingDir: call.Context.WorkingDir,
			Decision:   evaluation.Decision,
			Message:    evaluation.Message,
		}
	}

	start := time.Now()
	content, redirect, err := t.fetcher.FetchURLMarkdownContent(ctx, urlStr)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("WebFetch failed: %v", err)}, nil
	}

	// Handle cross-host redirect.
	if redirect != nil {
		message := fmt.Sprintf(`REDIRECT DETECTED: The URL redirects to a different host.

Original URL: %s
Redirect URL: %s
Status: %d %s

To complete your request, I need to fetch content from the redirected URL. Please use WebFetch again with these parameters:
- url: "%s"
- prompt: "%s"`, redirect.OriginalUrl, redirect.RedirectUrl, redirect.StatusCode, redirect.StatusText, redirect.RedirectUrl, input.Prompt)

		output := Output{
			Bytes:      len(message),
			Code:       redirect.StatusCode,
			CodeText:   redirect.StatusText,
			Result:     message,
			DurationMs: durationMs,
			URL:        urlStr,
		}
		return coretool.Result{
			Output: message,
			Meta: map[string]any{
				"data": output,
			},
		}, nil
	}

	// Process content with the prompt (minimal: just truncate for now).
	result := truncateMarkdown(content.Content, maxResultSizeChars)
	if input.Prompt != "" {
		// For the minimal implementation, append the prompt as a note since
		// secondary model processing is deferred.
		result = fmt.Sprintf("%s\n\n[User prompt: %s]", result, input.Prompt)
	}

	output := Output{
		Bytes:      content.Bytes,
		Code:       content.Code,
		CodeText:   content.CodeText,
		Result:     result,
		DurationMs: durationMs,
		URL:        urlStr,
	}

	logger.DebugCF("web_fetch_tool", "web fetch finished", map[string]any{
		"url":         urlStr,
		"code":        output.Code,
		"bytes":       output.Bytes,
		"duration_ms": output.DurationMs,
	})

	return coretool.Result{
		Output: result,
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"url": {
				Type:        coretool.ValueKindString,
				Description: "The URL to fetch content from.",
				Required:    true,
			},
			"prompt": {
				Type:        coretool.ValueKindString,
				Description: "The prompt to run on the fetched content.",
			},
		},
	}
}
