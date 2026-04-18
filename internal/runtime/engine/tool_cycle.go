package engine

import (
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// toolExecutionBatch stores one contiguous group of tool uses that share the same runtime concurrency mode.
type toolExecutionBatch struct {
	// concurrencySafe reports whether the entire batch may execute in parallel.
	concurrencySafe bool
	// toolUses preserves the original provider ordering for the batch.
	toolUses []model.ToolUse
}

// toolExecutionOutcome stores the completed result for one tool use while preserving its original order metadata.
type toolExecutionOutcome struct {
	// toolUse stores the original provider tool_use envelope.
	toolUse model.ToolUse
	// result stores the structured tool result returned by the executor.
	result coretool.Result
	// invokeErr stores any transport or execution error raised while invoking the tool.
	invokeErr error
}

// partitionToolUses groups contiguous tool uses into either parallel-safe or exclusive execution batches.
func partitionToolUses(toolUses []model.ToolUse, executor ToolExecutor) []toolExecutionBatch {
	if len(toolUses) == 0 {
		return nil
	}

	batches := make([]toolExecutionBatch, 0, len(toolUses))
	for _, toolUse := range toolUses {
		safe := false
		if executor != nil {
			safe = executor.IsConcurrencySafe(toolUse.Name)
		}

		if safe && len(batches) > 0 && batches[len(batches)-1].concurrencySafe {
			batches[len(batches)-1].toolUses = append(batches[len(batches)-1].toolUses, toolUse)
			continue
		}

		batches = append(batches, toolExecutionBatch{
			concurrencySafe: safe,
			toolUses:        []model.ToolUse{toolUse},
		})
	}
	return batches
}
