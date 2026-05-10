package repl

import (
	"context"
	"fmt"
	"sort"
	"strings"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// toolDescription describes the REPL tool's purpose for model-facing guidance.
const toolDescription = `Report whether REPL mode is enabled in the current runtime environment.

REPL mode controls which tools are directly accessible to the model vs. routed through the REPL. When REPL mode is on, certain tools (FileRead, FileWrite, FileEdit, Glob, Grep, Bash, NotebookEdit, Agent) are only accessible via REPL interactions.

Use this tool to check the current REPL mode status and understand which tools are affected.`

// Input is the typed request payload for the REPL tool.
// Zero-input: calling the tool without arguments reports current REPL mode.
type Input struct {
	// ShowTools requests a list of tools affected by REPL mode.
	ShowTools bool `json:"show_tools,omitempty"`
}

// REPLModeInfo contains the current REPL mode status.
type REPLModeInfo struct {
	// Enabled reports whether REPL mode is currently active.
	Enabled bool `json:"enabled"`
	// ReplOnlyToolNames lists the tools affected by REPL mode.
	ReplOnlyToolNames []string `json:"repl_only_tool_names,omitempty"`
}

// Output is the structured result returned by the REPL tool.
type Output struct {
	ReplMode REPLModeInfo `json:"repl_mode"`
	Message  string       `json:"message"`
}

// Tool implements the REPL tool.
type Tool struct{}

// NewTool constructs a REPL tool instance.
func NewTool() *Tool {
	return &Tool{}
}

// Name returns the stable registration name.
func (t *Tool) Name() string {
	return Name
}

// Description returns the tool summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return toolDescription
}

// InputSchema returns the REPL tool input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that REPL does not mutate external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that independent invocations may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke reports the current REPL mode status.
func (t *Tool) Invoke(_ context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("repl tool: nil receiver")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	enabled := IsReplModeEnabled()
	info := REPLModeInfo{
		Enabled: enabled,
	}

	var msg strings.Builder
	if enabled {
		msg.WriteString("REPL mode is enabled. Tools may be routed through the REPL.")
	} else {
		msg.WriteString("REPL mode is disabled. All tools are directly accessible.")
	}

	if input.ShowTools {
		toolNames := make([]string, 0, len(REPL_ONLY_TOOLS))
		for name := range REPL_ONLY_TOOLS {
			toolNames = append(toolNames, name)
		}
		sort.Strings(toolNames)
		info.ReplOnlyToolNames = toolNames

		msg.WriteString("\n\nTools affected by REPL mode:")
		msg.WriteString("\n" + strings.Join(toolNames, "\n"))
	}

	return coretool.Result{
		Output: msg.String(),
		Meta: map[string]any{
			"data": Output{
				ReplMode: info,
				Message:  msg.String(),
			},
		},
	}, nil
}

// inputSchema builds the declared input schema exposed to model providers.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"show_tools": {
				Type:        coretool.ValueKindBoolean,
				Description: "Set to true to list all tools affected by REPL mode.",
				Required:    false,
			},
		},
	}
}
