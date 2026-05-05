package coordinator

import (
	"context"
	"fmt"
	"strings"
)

// AgentInput holds the parameters for an Agent tool invocation.
// This is a coordinator-local mirror of agent.Input to avoid import cycles.
type AgentInput struct {
	// Description is a short (3-5 word) description of the task.
	Description string
	// Prompt is the task for the agent to perform.
	Prompt string
	// SubagentType is the type of specialized agent to use.
	SubagentType string
	// Model is an optional model override.
	Model string
	// Cwd overrides the working directory for the agent.
	Cwd string
}

// AgentOutput holds the result of an Agent tool invocation.
// This is a coordinator-local mirror of agent.Output to avoid import cycles.
type AgentOutput struct {
	// AgentID is the unique identifier of the spawned agent.
	AgentID string
	// AgentType is the type of the spawned agent.
	AgentType string
	// Content is the textual result from the agent.
	Content string
	// TotalToolUseCount is the total number of tool uses during the run.
	TotalToolUseCount int
	// TotalDurationMs is the wall-clock duration of the agent run.
	TotalDurationMs int
	// TotalTokens is the total number of tokens consumed.
	TotalTokens int
}

// AgentRunner is an interface for executing agent tasks.
// This allows the coordinator to use any agent runner implementation.
type AgentRunner interface {
	// Run executes an agent task and returns the output.
	Run(ctx context.Context, input AgentInput) (AgentOutput, error)
}

// TextBlock represents a text content block from an agent.
type TextBlock struct {
	Type string
	Text string
}

// AgentOutputRaw is the raw output format from the agent runner.
type AgentOutputRaw struct {
	AgentID           string
	AgentType         string
	Content           []TextBlock
	TotalToolUseCount int
	TotalDurationMs   int
	TotalTokens       int
}

// AgentRunnerRaw is an interface for executing agent tasks with raw output.
type AgentRunnerRaw interface {
	Run(ctx context.Context, input AgentInput) (AgentOutputRaw, error)
}

// RawToAgentRunnerAdapter wraps an AgentRunnerRaw to implement AgentRunner.
type RawToAgentRunnerAdapter struct {
	inner AgentRunnerRaw
}

// NewRawToAgentRunnerAdapter creates a new adapter.
func NewRawToAgentRunnerAdapter(r AgentRunnerRaw) *RawToAgentRunnerAdapter {
	return &RawToAgentRunnerAdapter{inner: r}
}

// Run executes an agent task and converts the output.
func (a *RawToAgentRunnerAdapter) Run(ctx context.Context, input AgentInput) (AgentOutput, error) {
	if a.inner == nil {
		return AgentOutput{}, fmt.Errorf("inner runner is nil")
	}

	rawOutput, err := a.inner.Run(ctx, input)
	if err != nil {
		return AgentOutput{}, err
	}

	// Convert raw output to simplified output
	var content string
	if len(rawOutput.Content) > 0 {
		var parts []string
		for _, block := range rawOutput.Content {
			parts = append(parts, block.Text)
		}
		content = strings.Join(parts, "")
	}

	return AgentOutput{
		AgentID:           rawOutput.AgentID,
		AgentType:         rawOutput.AgentType,
		Content:           content,
		TotalToolUseCount: rawOutput.TotalToolUseCount,
		TotalDurationMs:   rawOutput.TotalDurationMs,
		TotalTokens:       rawOutput.TotalTokens,
	}, nil
}

// AgentRunnerFromFactory is a factory function that creates an AgentRunner.
type AgentRunnerFromFactory func() AgentRunner

// FactoryAdapter wraps a factory function to implement the AgentRunner interface.
// It creates a new runner for each call to Run, matching the agent tool's pattern.
type FactoryAdapter struct {
	factory AgentRunnerFromFactory
}

// NewFactoryAdapter creates a new adapter that uses a factory to create runners.
func NewFactoryAdapter(factory AgentRunnerFromFactory) *FactoryAdapter {
	return &FactoryAdapter{factory: factory}
}

// Run creates a new runner from the factory and executes the agent task.
func (f *FactoryAdapter) Run(ctx context.Context, input AgentInput) (AgentOutput, error) {
	if f.factory == nil {
		return AgentOutput{}, fmt.Errorf("runner factory is nil")
	}
	runner := f.factory()
	if runner == nil {
		return AgentOutput{}, fmt.Errorf("factory returned nil runner")
	}
	return runner.Run(ctx, input)
}
