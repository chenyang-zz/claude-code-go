package enter_plan_mode

import (
	"context"
	"fmt"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

const (
	// Name is the stable registry identifier for the EnterPlanMode tool.
	Name = "EnterPlanMode"
)

// toolDescription merges the TS description() and prompt() into a single comprehensive
// model-facing text. It covers when to use EnterPlanMode, what happens in plan mode,
// examples, and important caveats.
const toolDescription = `Requests permission to enter plan mode for complex tasks requiring exploration and design.

EnterPlanMode should be used for:
- New feature implementation
- Tasks with multiple valid approaches
- Code modifications that affect existing behavior
- Architectural decisions
- Multi-file changes (>2-3 files)
- Unclear requirements
- User preferences matter

Do NOT use EnterPlanMode for:
- Single-line fixes
- Adding a single function with clear requirements
- Very specific detailed instructions given
- Pure research tasks

Plan mode allows you to explore the codebase and design an approach for the user's approval.
This tool REQUIRES user approval.

When in doubt, err on the side of planning.

IMPORTANT: Only use when the task requires planning implementation steps. For research tasks, do NOT use this tool.

Use AskUserQuestion if you need to clarify approaches before planning.

An empty result means the user declined to enter plan mode.`

// planModeOutputText is the guidance output returned to the model when plan mode is entered.
const planModeOutputText = `Entered plan mode. You should now focus on exploring the codebase and designing an implementation approach.

In plan mode, you should:
1. Thoroughly explore the codebase to understand existing patterns
2. Identify similar features and architectural approaches
3. Consider multiple approaches and their trade-offs
4. Use AskUserQuestion if you need to clarify the approach
5. Design a concrete implementation strategy
6. When ready, use ExitPlanMode to present your plan for approval

Remember: DO NOT write or edit any files yet. This is a read-only exploration and planning phase.`

// Output is the structured result returned by the EnterPlanMode tool.
type Output struct {
	// Message contains the plan mode guidance text for the model.
	Message string `json:"message"`
}

// Tool implements the EnterPlanMode tool.
type Tool struct{}

// NewTool constructs an EnterPlanMode tool instance.
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

// InputSchema returns an empty input contract — EnterPlanMode accepts no arguments.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{},
	}
}

// IsReadOnly reports that EnterPlanMode never mutates external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that independent invocations may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// RequiresUserInteraction reports that this tool requires user approval before the model can continue.
func (t *Tool) RequiresUserInteraction() bool {
	return true
}

// Invoke validates the empty input and returns plan mode guidance for the model.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("enter_plan_mode tool: nil receiver")
	}

	_, err := coretool.DecodeInput[struct{}](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	output := Output{
		Message: planModeOutputText,
	}

	return coretool.Result{
		Output: output.Message,
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}
