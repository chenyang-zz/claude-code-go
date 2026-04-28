package delete

import (
	"context"
	"fmt"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	cronshared "github.com/sheepzhao/claude-code-go/internal/services/tools/cron/shared"
)

const (
	// Name is the stable registry identifier for the CronDelete tool.
	Name = "CronDelete"
)

// toolDescription describes what CronDelete does and when to use it.
const toolDescription = `Cancel a scheduled cron job by ID. Removes it from the in-memory session store.`

// Input is the typed request payload for the CronDelete tool.
type Input struct {
	// ID is the job identifier returned by CronCreate.
	ID string `json:"id"`
}

// Output is the structured result returned by the CronDelete tool.
type Output struct {
	// ID is the identifier of the cancelled job.
	ID string `json:"id"`
}

// Tool implements the CronDelete tool.
type Tool struct {
	store *cronshared.Store
}

// NewTool constructs a CronDelete tool instance with the shared task store.
func NewTool(store *cronshared.Store) *Tool {
	return &Tool{store: store}
}

// Name returns the stable registration name.
func (t *Tool) Name() string {
	return Name
}

// Description returns the tool summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return toolDescription
}

// InputSchema returns the CronDelete input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that CronDelete mutates task state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that independent invocations may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// RequiresUserInteraction reports that this tool requires user approval before cancelling.
func (t *Tool) RequiresUserInteraction() bool {
	return true
}

// Invoke validates that the given job ID exists and removes it from the shared store.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("cron_delete tool: nil receiver")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	if err := t.store.Delete(input.ID); err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	output := Output{ID: input.ID}

	return coretool.Result{
		Output: fmt.Sprintf("Cancelled job %s.", input.ID),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// inputSchema builds the declared input schema exposed to model providers.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"id": {
				Type:        coretool.ValueKindString,
				Description: "Job ID returned by CronCreate.",
				Required:    true,
			},
		},
	}
}
