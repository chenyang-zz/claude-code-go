package exit_plan_mode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier for the ExitPlanMode tool.
	Name = "ExitPlanMode"
)

// toolDescription merges the TS description() and prompt() into a single
// model-facing text. It explains when and how to use ExitPlanMode.
const toolDescription = `Prompts the user to exit plan mode and start coding. Use this tool when you are in plan mode and have finished writing your plan to the plan file and are ready for user approval.

## How This Tool Works
- You should have already written your plan to the plan file specified in the plan mode system message
- This tool does NOT take the plan content as a parameter - it will read the plan from the file you wrote
- This tool simply signals that you're done planning and ready for the user to review and approve
- The user will see the contents of your plan file when they review it

## When to Use This Tool
IMPORTANT: Only use this tool when the task requires planning the implementation steps of a task that requires writing code. For research tasks where you're gathering information, searching files, reading files or in general trying to understand the codebase - do NOT use this tool.

## Before Using This Tool
Ensure your plan is complete and unambiguous:
- If you have unresolved questions about requirements or approach, use AskUserQuestion first (in earlier phases)
- Once your plan is finalized, use THIS tool to request approval

**Important:** Do NOT use AskUserQuestion to ask "Is this plan okay?" or "Should I proceed?" - that's exactly what THIS tool does. ExitPlanMode inherently requests user approval of your plan.`

// AllowedPrompt represents one prompt-based permission entry requested by the plan.
type AllowedPrompt struct {
	// Tool is the tool this prompt applies to (e.g. "Bash").
	Tool string `json:"tool"`
	// Prompt is a semantic description of the action (e.g. "run tests").
	Prompt string `json:"prompt"`
}

// Input is the typed request payload for the ExitPlanMode tool.
type Input struct {
	// AllowedPrompts carries prompt-based permissions requested by the plan.
	AllowedPrompts []AllowedPrompt `json:"allowedPrompts,omitempty"`
}

// Output is the structured result returned by the ExitPlanMode tool.
type Output struct {
	// Plan holds the plan content read from the plan file.
	Plan string `json:"plan"`
	// IsAgent reports whether the tool was invoked from an agent context.
	IsAgent bool `json:"isAgent"`
	// FilePath is the path to the plan file that was read.
	FilePath string `json:"filePath,omitempty"`
}

// Tool implements the ExitPlanMode tool.
type Tool struct{}

// NewTool constructs an ExitPlanMode tool instance.
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

// InputSchema returns the ExitPlanMode input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that ExitPlanMode may write plan file updates and exit state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that independent invocations may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// RequiresUserInteraction reports that this tool requires user confirmation to exit plan mode.
func (t *Tool) RequiresUserInteraction() bool {
	return true
}

// Invoke reads the plan file from the working directory and returns the structured output.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("exit_plan_mode tool: nil receiver")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	_ = input // allowedPrompts is accepted but not acted on in minimum implementation

	plan, filePath := readPlanFile(call.Context.WorkingDir)

	output := Output{
		Plan:     plan,
		IsAgent:  false,
		FilePath: filePath,
	}

	return coretool.Result{
		Output: formatOutputText(plan, filePath),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// inputSchema builds the declared input schema exposed to model providers.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"allowedPrompts": {
				Type:        coretool.ValueKindArray,
				Description: "Prompt-based permissions needed to implement the plan. These describe categories of actions rather than specific commands.",
				Items: &coretool.FieldSchema{
					Type:        coretool.ValueKindObject,
					Description: "A permission entry with tool name and semantic prompt description.",
				},
			},
		},
	}
}

// readPlanFile attempts to read a plan file from the .claude/plans directory under
// the given working directory. Returns the plan content and file path, or empty strings.
func readPlanFile(workingDir string) (string, string) {
	plansDir := filepath.Join(workingDir, ".claude", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		logger.DebugCF("exit_plan_mode", "cannot read plans directory", map[string]any{
			"plans_dir": plansDir,
			"error":     err.Error(),
		})
		return "", ""
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(plansDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			logger.DebugCF("exit_plan_mode", "cannot read plan file", map[string]any{
				"file_path": filePath,
				"error":     err.Error(),
			})
			continue
		}

		logger.DebugCF("exit_plan_mode", "plan file read", map[string]any{
			"file_path":    filePath,
			"content_size": len(data),
		})
		return string(data), filePath
	}

	return "", ""
}

// formatOutputText builds the model-facing output for the plan approval result.
func formatOutputText(plan, filePath string) string {
	if plan == "" {
		return "User has approved exiting plan mode. You can now proceed."
	}

	var b strings.Builder
	b.WriteString("User has approved your plan. You can now start coding. Start with updating your todo list if applicable\n\n")
	if filePath != "" {
		fmt.Fprintf(&b, "Your plan has been saved to: %s\n", filePath)
		b.WriteString("You can refer back to it if needed during implementation.\n\n")
	}
	b.WriteString("## Approved Plan:\n")
	b.WriteString(plan)
	return b.String()
}
