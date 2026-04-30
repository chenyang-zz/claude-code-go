// Package synthetic_output implements the StructuredOutput tool that returns
// structured JSON output for non-interactive sessions.
package synthetic_output

import (
	"context"
	"encoding/json"
	"fmt"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

const (
	// Name is the tool identifier registered in the tool set.
	Name = "StructuredOutput"
	// toolDescription provides a short summary for the tool.
	toolDescription = "Return structured output in the requested format"
)

// Input accepts any JSON object as passthrough input.
type Input map[string]any

// Output contains the structured output result.
type Output struct {
	// Success indicates whether the structured output was processed.
	Success bool `json:"success"`
	// Output contains the human-readable confirmation message.
	Output string `json:"output"`
	// StructuredOutput holds the original input returned as structured data.
	StructuredOutput Input `json:"structuredOutput,omitempty"`
}

// Tool implements the coretool.Tool interface for structured output passthrough.
type Tool struct{}

// NewTool returns a new StructuredOutput tool instance.
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

// InputSchema returns an empty schema that accepts any JSON object.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{},
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

// Invoke processes the tool call and returns structured output confirmation.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("synthetic_output: nil receiver")
	}

	input := make(Input)
	if call.Input != nil {
		for k, v := range call.Input {
			input[k] = v
		}
	}

	output := Output{
		Success:          true,
		Output:           "Structured output provided successfully",
		StructuredOutput: input,
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
		return fmt.Sprintf(`{"success":true,"output":"%s"}`, output.Output)
	}
	return string(data)
}
