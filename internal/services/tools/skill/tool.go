package skill

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier for the Skill tool.
	Name = "Skill"
	// toolDescription is the model-facing description of the Skill tool.
	toolDescription = `Execute a slash command skill. Use this when you need to invoke a skill/command like /help, /clear, /compact, or any user-defined skill.

Usage notes:
- Provide the skill name without the leading slash (e.g., "commit" not "/commit")
- Optional args can be passed as a string
- If the skill is not found, an error is returned with details`
)

// SharedRegistry carries the command registry set by bootstrap after tool creation.
// It mirrors the cronStore / worktreeManager package-level shared state pattern.
var SharedRegistry command.Registry

// Input is the typed request payload for the Skill tool.
type Input struct {
	// Skill is the name of the skill/command to invoke (without leading slash).
	Skill string `json:"skill"`
	// Args holds optional arguments for the skill.
	Args string `json:"args,omitempty"`
}

// Output is the structured result returned by the Skill tool.
type Output struct {
	// Success indicates whether the skill was found and executed.
	Success bool `json:"success"`
	// CommandName is the name of the invoked command.
	CommandName string `json:"commandName"`
	// Output holds the command execution result text.
	Output string `json:"output"`
}

// Tool implements the minimum migrated Skill tool.
// It resolves slash-command skills from SharedRegistry and executes them.
type Tool struct{}

// NewTool constructs a Skill tool instance.
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

// InputSchema returns the Skill input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"skill": {
				Type:        coretool.ValueKindString,
				Description: "The skill name. E.g., \"commit\", \"review-pr\", or \"pdf\".",
				Required:    true,
			},
			"args": {
				Type:        coretool.ValueKindString,
				Description: "Optional arguments for the skill.",
				Required:    false,
			},
		},
	}
}

// IsReadOnly reports that Skill execution may mutate external state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that only one skill should run at a time.
func (t *Tool) IsConcurrencySafe() bool {
	return false
}

// RequiresUserInteraction reports that skill execution should be deferred for user confirmation.
func (t *Tool) RequiresUserInteraction() bool {
	return true
}

// Invoke validates the skill name, looks up the command from SharedRegistry, and executes it.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("skill tool: nil receiver")
	}

	if SharedRegistry == nil {
		return coretool.Result{
			Error: "skill tool: command registry not initialized",
		}, nil
	}

	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	// Normalize skill name: trim whitespace and remove leading slash
	normalizedName := strings.TrimSpace(input.Skill)
	normalizedName = strings.TrimPrefix(normalizedName, "/")

	if normalizedName == "" {
		return coretool.Result{
			Error: fmt.Sprintf("Invalid skill format: %q", input.Skill),
		}, nil
	}

	// Look up command from shared registry
	cmd, found := SharedRegistry.Get(normalizedName)
	if !found {
		return coretool.Result{
			Error: fmt.Sprintf("Unknown skill: %s", normalizedName),
		}, nil
	}

	logger.DebugCF("skill", "executing skill via SkillTool", map[string]any{
		"command": normalizedName,
		"args":    input.Args,
	})

	// Build command args from the input args string
	cmdArgs := command.Args{
		Raw:     splitArgs(input.Args),
		RawLine: input.Args,
	}

	// Execute the command
	result, err := cmd.Execute(ctx, cmdArgs)
	if err != nil {
		logger.WarnCF("skill", "skill execution failed", map[string]any{
			"command": normalizedName,
			"error":   err.Error(),
		})
		return coretool.Result{
			Error: fmt.Sprintf("Skill %q execution failed: %s", normalizedName, err.Error()),
		}, nil
	}

	output := Output{
		Success:     true,
		CommandName: normalizedName,
		Output:      result.Output,
	}

	logger.DebugCF("skill", "skill executed successfully", map[string]any{
		"command": normalizedName,
	})

	return coretool.Result{
		Output: formatOutputText(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// splitArgs performs minimal argument splitting from a raw args string.
// Splits on spaces while preserving quoted substrings.
func splitArgs(raw string) []string {
	if raw == "" {
		return nil
	}

	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		switch {
		case ch == '"' || ch == '\'':
			if inQuote && ch == quoteChar {
				inQuote = false
				quoteChar = 0
			} else if !inQuote {
				inQuote = true
				quoteChar = ch
			} else {
				current.WriteByte(ch)
			}
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// formatOutputText builds a human-readable summary for the model.
func formatOutputText(output Output) string {
	if !output.Success {
		return "Skill execution failed."
	}
	if output.Output != "" {
		return fmt.Sprintf("Skill %q executed:\n\n%s", output.CommandName, output.Output)
	}
	return fmt.Sprintf("Skill %q executed successfully.", output.CommandName)
}
