package magicdocs

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// SubagentRunner abstracts the ability to launch a subagent for Magic Docs updates.
// The implementation is provided by bootstrap and uses the AgentTool runner internally.
type SubagentRunner interface {
	// RunSubagent launches an asynchronous subagent that only has access to the
	// FileEditTool constrained to the given file path. The subagent receives the
	// update prompt and the conversation context and runs to completion.
	RunSubagent(ctx context.Context, filePath string, updatePrompt string, messages []message.Message) error
}

// magicDocsAgentType is the agent type identifier for the Magic Docs updater subagent.
const magicDocsAgentType = "magic-docs"

// MagicDocsFileEditTool is the name of the FileEditTool as known to the tool registry.
const MagicDocsFileEditTool = "file_edit"
