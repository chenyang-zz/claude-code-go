package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/compact"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/core/transcript"
	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
)

const (
	// defaultMaxMcpOutputTokens mirrors the TS-side default token cap for MCP
	// outputs when no explicit override is present.
	defaultMaxMcpOutputTokens = 25000
	// envMaxMcpOutputTokens allows users to override the default MCP output cap.
	envMaxMcpOutputTokens = "MAX_MCP_OUTPUT_TOKENS"
	// envEnableLargeMcpOutputs toggles file persistence for large outputs.
	envEnableLargeMcpOutputs = "ENABLE_MCP_LARGE_OUTPUT_FILES"
	// mcpOutputDirName is the stable directory name used for persisted outputs.
	mcpOutputDirName = "mcp-output"
	// mcpImageTokenEstimate approximates the token cost of a binary image block
	// when the result must stay inline.
	mcpImageTokenEstimate = 1600
)

// mcpResultKind describes how one MCP result was normalized.
type mcpResultKind string

const (
	// resultKindStructuredContent marks a structuredContent payload.
	resultKindStructuredContent mcpResultKind = "structured_content"
	// resultKindContentArray marks a text-only content array payload.
	resultKindContentArray mcpResultKind = "content_array"
	// resultKindInlineText marks a payload that should remain inline text.
	resultKindInlineText mcpResultKind = "inline_text"
)

// normalizedMcpResult captures the bridge-ready representation of an MCP tool
// result before persistence or truncation decisions are applied.
type normalizedMcpResult struct {
	// Kind records which source branch produced the output.
	Kind mcpResultKind
	// InlineText is the human-readable text representation used for small
	// results and truncation fallbacks.
	InlineText string
	// PersistBytes stores the exact bytes to write when the large-output path
	// decides to persist the result.
	PersistBytes []byte
	// Persistable reports whether the output can be safely written to disk.
	Persistable bool
	// FormatDescription describes the persisted format for the user-facing hint.
	FormatDescription string
	// TokenEstimate stores the rough size estimate used for the large-output decision.
	TokenEstimate int
}

// buildToolResultOutput normalizes an MCP response and applies large-output
// storage or truncation decisions.
func buildToolResultOutput(serverName string, mcpTool client.Tool, call coretool.Call, result *client.CallToolResult) coretool.Result {
	normalized, err := normalizeMcpResult(result)
	if err != nil {
		return coretool.Result{Error: err.Error()}
	}

	if normalized.TokenEstimate <= getMaxMcpOutputTokens() {
		return coretool.Result{
			Output: normalized.InlineText,
			Meta: map[string]any{
				"server":      serverName,
				"tool":        mcpTool.Name,
				"result_kind": string(normalized.Kind),
			},
		}
	}

	if isFalsyLargeOutputToggle() || !normalized.Persistable {
		return coretool.Result{
			Output: truncateMcpInlineText(normalized.InlineText, normalized.TokenEstimate),
			Meta: map[string]any{
				"server":         serverName,
				"tool":           mcpTool.Name,
				"result_kind":    string(normalized.Kind),
				"truncated":      true,
				"persistable":    normalized.Persistable,
				"token_estimate": normalized.TokenEstimate,
			},
		}
	}

	filepath, err := persistLargeMcpOutput(serverName, mcpTool.Name, call, normalized)
	if err != nil {
		return coretool.Result{
			Output: buildLargeMcpOutputFallback(normalized.InlineText, normalized.TokenEstimate, err),
			Meta: map[string]any{
				"server":        serverName,
				"tool":          mcpTool.Name,
				"result_kind":   string(normalized.Kind),
				"persist_error": err.Error(),
			},
		}
	}

	return coretool.Result{
		Output: buildLargeMcpOutputInstructions(filepath, normalized.TokenEstimate, normalized.FormatDescription),
		Meta: map[string]any{
			"server":         serverName,
			"tool":           mcpTool.Name,
			"result_kind":    string(normalized.Kind),
			"persisted":      true,
			"output_path":    filepath,
			"token_estimate": normalized.TokenEstimate,
			"format":         normalized.FormatDescription,
		},
	}
}

// normalizeMcpResult converts the MCP wire result into a stable bridge shape.
func normalizeMcpResult(result *client.CallToolResult) (normalizedMcpResult, error) {
	if result == nil {
		return normalizedMcpResult{}, fmt.Errorf("mcp tool result: nil result")
	}

	if result.StructuredContent != nil {
		payload, err := json.MarshalIndent(result.StructuredContent, "", "  ")
		if err != nil {
			return normalizedMcpResult{}, fmt.Errorf("mcp tool result: marshal structuredContent: %w", err)
		}
		return normalizedMcpResult{
			Kind:              resultKindStructuredContent,
			InlineText:        string(payload),
			PersistBytes:      payload,
			Persistable:       true,
			FormatDescription: "JSON",
			TokenEstimate:     compact.EstimateTokensForText(string(payload)),
		}, nil
	}

	if len(result.Content) == 0 {
		return normalizedMcpResult{}, fmt.Errorf("mcp tool result: unexpected response format")
	}

	inlineText := contentToString(result.Content)
	persistable := allContentItemsAreText(result.Content)

	normalized := normalizedMcpResult{
		Kind:              resultKindInlineText,
		InlineText:        inlineText,
		Persistable:       persistable,
		FormatDescription: "Plain text",
	}

	if persistable {
		payload, err := json.MarshalIndent(result.Content, "", "  ")
		if err != nil {
			return normalizedMcpResult{}, fmt.Errorf("mcp tool result: marshal content array: %w", err)
		}
		normalized.Kind = resultKindContentArray
		normalized.PersistBytes = payload
		normalized.FormatDescription = "JSON array"
	}

	normalized.TokenEstimate = estimateContentTokens(result.Content, inlineText, persistable, normalized.PersistBytes)

	return normalized, nil
}

// estimateContentTokens produces a rough token estimate for an MCP content array.
func estimateContentTokens(items []client.ContentItem, inlineText string, persistable bool, persistBytes []byte) int {
	if persistable {
		return compact.EstimateTokensForText(string(persistBytes))
	}

	total := compact.EstimateTokensForText(inlineText)
	for _, item := range items {
		if item.Type != "text" {
			total += estimateContentItemTokens(item)
		}
	}
	return total
}

// estimateContentItemTokens estimates the token cost of one MCP content item.
func estimateContentItemTokens(item client.ContentItem) int {
	switch item.Type {
	case "text":
		return compact.EstimateTokensForText(item.Text)
	case "image":
		if item.Data != "" {
			return len(item.Data) / 8
		}
		return mcpImageTokenEstimate
	default:
		if item.Text != "" {
			return compact.EstimateTokensForText(item.Text)
		}
		return 0
	}
}

// allContentItemsAreText reports whether the MCP result can be safely persisted
// as JSON text blocks without losing non-text content.
func allContentItemsAreText(items []client.ContentItem) bool {
	for _, item := range items {
		if item.Type != "text" {
			return false
		}
	}
	return true
}

// getMaxMcpOutputTokens resolves the output token ceiling for MCP results.
func getMaxMcpOutputTokens() int {
	if raw := strings.TrimSpace(os.Getenv(envMaxMcpOutputTokens)); raw != "" {
		var parsed int
		if _, err := fmt.Sscanf(raw, "%d", &parsed); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultMaxMcpOutputTokens
}

// isFalsyLargeOutputToggle reports whether the large-output persistence toggle
// is explicitly disabled.
func isFalsyLargeOutputToggle() bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(envEnableLargeMcpOutputs)))
	switch raw {
	case "", "0", "false", "no", "off":
		return raw != ""
	default:
		return false
	}
}

// persistLargeMcpOutput writes the normalized large result to a stable file.
func persistLargeMcpOutput(serverName string, toolName string, call coretool.Call, normalized normalizedMcpResult) (string, error) {
	outputPath := buildMcpOutputPath(call.Context.WorkingDir, serverName, toolName, call.ID, normalized)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, normalized.PersistBytes, 0o600); err != nil {
		return "", fmt.Errorf("write output file: %w", err)
	}

	return outputPath, nil
}

// buildMcpOutputPath constructs the persisted output path for one MCP result.
func buildMcpOutputPath(workingDir, serverName, toolName, callID string, normalized normalizedMcpResult) string {
	scope := transcript.SanitizePath(workingDir)
	if strings.TrimSpace(scope) == "" {
		scope = "default"
	}
	server := transcript.SanitizePath(serverName)
	if strings.TrimSpace(server) == "" {
		server = "mcp"
	}
	tool := transcript.SanitizePath(toolName)
	if strings.TrimSpace(tool) == "" {
		tool = "tool"
	}
	callComponent := transcript.SanitizePath(callID)
	if strings.TrimSpace(callComponent) == "" {
		callComponent = "result"
	}

	ext := "txt"
	if normalized.FormatDescription == "JSON" || normalized.FormatDescription == "JSON array" {
		ext = "json"
	}

	return filepath.Join(transcript.GetClaudeConfigHomeDir(), mcpOutputDirName, scope, server, tool, callComponent+"."+ext)
}

// buildLargeMcpOutputInstructions explains where the output was saved and how
// the model should read it back in chunks.
func buildLargeMcpOutputInstructions(filepath string, tokenEstimate int, formatDescription string) string {
	return fmt.Sprintf(
		"Result (%d tokens) exceeds maximum allowed tokens (%d). Output has been saved to %s.\nFormat: %s\nUse the Read tool with offset and limit to read specific portions of the file. If the file is structured, inspect it incrementally until you have read the full content before summarizing it.",
		tokenEstimate,
		getMaxMcpOutputTokens(),
		filepath,
		formatDescription,
	)
}

// buildLargeMcpOutputFallback explains that persistence failed and the bridge
// had to fall back to truncation.
func buildLargeMcpOutputFallback(inlineText string, tokenEstimate int, err error) string {
	if err == nil {
		return truncateMcpInlineText(inlineText, tokenEstimate)
	}

	return fmt.Sprintf(
		"Result (%d tokens) exceeds maximum allowed tokens (%d). Failed to save output to file: %v. %s",
		tokenEstimate,
		getMaxMcpOutputTokens(),
		err,
		truncateMcpInlineText(inlineText, tokenEstimate),
	)
}

// truncateMcpInlineText trims a long inline result and adds a stable warning.
func truncateMcpInlineText(inlineText string, tokenEstimate int) string {
	maxChars := getMaxMcpOutputTokens() * 4
	if len(inlineText) <= maxChars {
		return inlineText
	}
	return inlineText[:maxChars] + fmt.Sprintf("\n\n[OUTPUT TRUNCATED - exceeded %d token limit]", tokenEstimate)
}
