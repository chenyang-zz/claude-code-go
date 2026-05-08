package analytics

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// SanitizeToolName redacts MCP tool names to avoid PII exposure.
// Built-in tool names are preserved; MCP tools (mcp__<server>__<tool>) are
// redacted to "mcp_tool".
func SanitizeToolName(toolName string) string {
	if strings.HasPrefix(toolName, "mcp__") {
		return "mcp_tool"
	}
	return toolName
}

// ToolDetailsLoggingEnabled checks if detailed tool name logging is enabled
// for OTel events. Disabled by default to protect PII.
func ToolDetailsLoggingEnabled() bool {
	return false // Go side has no OTel integration; plumb env var if needed
}

// AnalyticsToolDetailsLoggingEnabled checks if MCP tool name logging is enabled
// for analytics events based on MCP server type and URL.
func AnalyticsToolDetailsLoggingEnabled(mcpServerType, mcpServerBaseURL string) bool {
	if mcpServerType == "claudeai-proxy" {
		return true
	}
	if mcpServerBaseURL != "" && isOfficialMCPURL(mcpServerBaseURL) {
		return true
	}
	// NOTE: local-agent check is not implemented on Go side.
	// The caller should pass true if running in local-agent mode.
	return false
}

// isOfficialMCPURL checks whether the given URL matches the official MCP registry.
// This is a simplified check; the TS side uses a dedicated official registry module.
func isOfficialMCPURL(url string) bool {
	// Official MCP servers are hosted under known domains.
	// This is a minimal implementation — expand as needed.
	return strings.Contains(url, "mcp.googleapis.com") ||
		strings.Contains(url, "api.claude.ai/mcp")
}

// MCPToolDetails returns MCP server and tool names for analytics payloads
// if the logging gate passes. Returns nil otherwise.
func MCPToolDetails(toolName string, mcpServerType, mcpServerBaseURL string) map[string]string {
	details := ExtractMCPToolDetails(toolName)
	if details == nil {
		return nil
	}
	if !AnalyticsToolDetailsLoggingEnabled(mcpServerType, mcpServerBaseURL) {
		return nil
	}
	return details
}

// ExtractMCPToolDetails extracts MCP server and tool names from a full MCP tool name.
// Tool names follow the format: mcp__<server>__<tool>
func ExtractMCPToolDetails(toolName string) map[string]string {
	if !strings.HasPrefix(toolName, "mcp__") {
		return nil
	}

	// Format: mcp__<server>__<tool>
	parts := strings.Split(toolName, "__")
	if len(parts) < 3 {
		return nil
	}

	serverName := parts[1]
	// Tool name may contain __ so rejoin remaining parts
	mcpToolName := strings.Join(parts[2:], "__")

	if serverName == "" || mcpToolName == "" {
		return nil
	}

	return map[string]string{
		"mcpServerName": serverName,
		"mcpToolName":   mcpToolName,
	}
}

// ExtractSkillName extracts the skill name from a Skill tool input.
// Returns the skill name if this is a Skill tool call, empty string otherwise.
func ExtractSkillName(toolName string, input any) string {
	if toolName != "Skill" {
		return ""
	}

	inputMap, ok := input.(map[string]any)
	if !ok {
		return ""
	}

	skill, ok := inputMap["skill"]
	if !ok {
		return ""
	}

	skillStr, ok := skill.(string)
	if !ok {
		return ""
	}

	return skillStr
}

// Tool input truncation constants.
const (
	toolInputStringTruncateAt    = 512
	toolInputStringTruncateTo    = 128
	toolInputMaxJSONChars        = 4 * 1024
	toolInputMaxCollectionItems  = 20
	toolInputMaxDepth            = 2
)

// truncateToolInputValue recursively truncates tool input values.
// Long strings are shortened, deep nesting is capped, and collections are limited.
func truncateToolInputValue(value any, depth int) any {
	if depth > toolInputMaxDepth {
		return "<nested>"
	}

	switch v := value.(type) {
	case string:
		if utf8.RuneCountInString(v) > toolInputStringTruncateAt {
			runes := []rune(v)
			truncated := string(runes[:min(len(runes), toolInputStringTruncateTo)])
			return truncated + "…[" + itoa(len(runes)) + " chars]"
		}
		return v
	case float64, float32, bool, nil:
		return v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v
	case []any:
		mapped := make([]any, 0, min(len(v), toolInputMaxCollectionItems))
		for i, item := range v {
			if i >= toolInputMaxCollectionItems {
				break
			}
			mapped = append(mapped, truncateToolInputValue(item, depth+1))
		}
		if len(v) > toolInputMaxCollectionItems {
			mapped = append(mapped, "…["+itoa(len(v))+" items]")
		}
		return mapped
	case map[string]any:
		// Skip internal marker keys (starting with underscore)
		mapped := make(map[string]any)
		count := 0
		for k, val := range v {
			if count >= toolInputMaxCollectionItems {
				mapped["…"] = itoa(len(v)) + " keys"
				break
			}
			if strings.HasPrefix(k, "_") {
				continue
			}
			mapped[k] = truncateToolInputValue(val, depth+1)
			count++
		}
		return mapped
	default:
		return "<unknown>"
	}
}

// ExtractToolInputForTelemetry serialises tool input arguments for telemetry.
// Returns the serialised and truncated tool input, or empty string when
// tool detail logging is not enabled.
func ExtractToolInputForTelemetry(input any) string {
	if !ToolDetailsLoggingEnabled() {
		return ""
	}
	truncated := truncateToolInputValue(input, 0)
	b, err := json.Marshal(truncated)
	if err != nil {
		return ""
	}
	if len(b) > toolInputMaxJSONChars {
		b = append(b[:toolInputMaxJSONChars], []byte("…[truncated]")...)
	}
	return string(b)
}

// MaxFileExtensionLength is the maximum length for logged file extensions.
var maxFileExtensionLength = 10

// FileExtensionForAnalytics extracts and sanitises a file extension for analytics logging.
// Returns "other" for extensions exceeding the max length to avoid logging
// potentially sensitive data (like hash-based filenames).
// Dotfiles (files starting with '.') return empty to match TS path.extname behaviour.
func FileExtensionForAnalytics(filePath string) string {
	ext := filepath.Ext(filePath)
	if ext == "" || ext == "." {
		return ""
	}

	extension := strings.ToLower(strings.TrimPrefix(ext, "."))
	// Skip dotfiles (where the "extension" is the entire basename, e.g. ".hidden")
	base := filepath.Base(filePath)
	if base == ext {
		return ""
	}
	if len(extension) > maxFileExtensionLength {
		return "other"
	}

	return extension
}

// fileCommands are Bash commands we extract file extensions from.
var fileCommands = map[string]bool{
	"rm": true, "mv": true, "cp": true, "touch": true, "mkdir": true,
	"chmod": true, "chown": true, "cat": true, "head": true, "tail": true,
	"sort": true, "stat": true, "diff": true, "wc": true, "grep": true,
	"rg": true, "sed": true,
}

// FileExtensionsFromBashCommand extracts file extensions from a bash command for analytics.
// Best-effort: splits on operators and whitespace, extracts extensions
// from non-flag args of allowed commands.
func FileExtensionsFromBashCommand(command string, simulatedSedEditFilePath string) string {
	if !strings.Contains(command, ".") && simulatedSedEditFilePath == "" {
		return ""
	}

	var result []string
	seen := make(map[string]bool)

	if simulatedSedEditFilePath != "" {
		ext := FileExtensionForAnalytics(simulatedSedEditFilePath)
		if ext != "" && !seen[ext] {
			seen[ext] = true
			result = append(result, ext)
		}
	}

	// Split on compound operators: &&, ||, ;, |
	for _, subcmd := range splitCompoundOperators(command) {
		subcmd = strings.TrimSpace(subcmd)
		if subcmd == "" {
			continue
		}
		tokens := strings.Fields(subcmd)
		if len(tokens) < 2 {
			continue
		}

		firstToken := tokens[0]
		slashIdx := strings.LastIndex(firstToken, "/")
		baseCmd := firstToken
		if slashIdx >= 0 {
			baseCmd = firstToken[slashIdx+1:]
		}
		if !fileCommands[baseCmd] {
			continue
		}

		for _, arg := range tokens[1:] {
			if len(arg) > 0 && arg[0] == '-' {
				continue
			}
			ext := FileExtensionForAnalytics(arg)
			if ext != "" && !seen[ext] {
				seen[ext] = true
				result = append(result, ext)
			}
		}
	}

	return strings.Join(result, ",")
}

func splitCompoundOperators(command string) []string {
	// Split on: &&, ||, ;, |
	// Simple split; the TS side uses regex \s*(?:&&|\|\||[;|])\s*
	var parts []string
	current := strings.Builder{}
	i := 0
	for i < len(command) {
		if i+1 < len(command) {
			pair := command[i : i+2]
			if pair == "&&" || pair == "||" {
				trimmed := strings.TrimSpace(current.String())
				if trimmed != "" {
					parts = append(parts, trimmed)
				}
				current.Reset()
				i += 2
				continue
			}
		}
		c := command[i]
		if c == ';' || c == '|' {
			trimmed := strings.TrimSpace(current.String())
			if trimmed != "" {
				parts = append(parts, trimmed)
			}
			current.Reset()
			i++
			continue
		}
		current.WriteByte(c)
		i++
	}
	trimmed := strings.TrimSpace(current.String())
	if trimmed != "" {
		parts = append(parts, trimmed)
	}
	return parts
}

// itoa is a simple int-to-string converter avoiding strconv import for basic cases.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// not needed — uses built-in min (Go 1.21+)
