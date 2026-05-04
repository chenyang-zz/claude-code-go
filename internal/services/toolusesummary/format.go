package toolusesummary

import (
	"encoding/json"
	"fmt"
	"strings"
)

// formatToolBatch renders the tool batch as the three-line plain-text format
// expected by the Haiku prompt:
//
//	Tool: <name>
//	Input: <truncated json>
//	Output: <truncated json>
//
// Multiple tools are joined with a blank line ("\n\n"). The input/output
// fields are JSON-encoded and capped at truncationLimit characters.
func formatToolBatch(tools []ToolInfo) string {
	if len(tools) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tools))
	for _, tool := range tools {
		inputStr := truncateJSON(tool.Input, truncationLimit)
		outputStr := truncateJSON(tool.Output, truncationLimit)
		parts = append(parts, fmt.Sprintf("Tool: %s\nInput: %s\nOutput: %s",
			tool.Name, inputStr, outputStr))
	}
	return strings.Join(parts, "\n\n")
}

// truncateJSON marshals value as JSON and truncates the result at maxLength
// characters using a trailing "..." sentinel. Returns "[unable to serialize]"
// when the value cannot be marshalled (e.g. contains func or chan fields).
//
// Mirrors the TS truncateJson behaviour: when len(json) <= maxLength the
// raw JSON is returned, otherwise the prefix is sliced to maxLength-3 and
// "..." is appended so the final string length equals maxLength.
func truncateJSON(value any, maxLength int) string {
	if maxLength <= 3 {
		return "..."
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "[unable to serialize]"
	}
	if len(data) <= maxLength {
		return string(data)
	}
	return string(data[:maxLength-3]) + "..."
}
