package query

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	runtimeengine "github.com/sheepzhao/claude-code-go/internal/runtime/engine"
)

// QueryEngine holds configuration for creating a QueryEngine.
type QueryEngineConfig struct {
	// Config holds query-level configuration.
	Config QueryConfig
	// Deps holds I/O dependencies.
	Deps QueryDeps
	// Runtime is the underlying engine Runtime to delegate to.
	Runtime *runtimeengine.Runtime
}

// QueryEngine owns the query lifecycle and session state for a conversation.
// It provides a simplified API for programmatic/SDK query execution,
// wrapping the engine Runtime for multi-turn usage.
//
// One QueryEngine per conversation. Each SubmitMessage call starts a new
// turn within the same conversation. State (messages) persists across turns.
type QueryEngine struct {
	config   QueryEngineConfig
	messages []message.Message
}

// NewQueryEngine creates a QueryEngine with the given configuration.
func NewQueryEngine(cfg QueryEngineConfig) *QueryEngine {
	return &QueryEngine{
		config: cfg,
	}
}

// Messages returns the current conversation message history.
func (qe *QueryEngine) Messages() []message.Message {
	return qe.messages
}

// SessionID returns the session ID associated with this query engine.
func (qe *QueryEngine) SessionID() string {
	return qe.config.Config.SessionID
}

// SubmitMessage submits a user message and runs one query turn.
// The conversation state (messages) is maintained across calls.
func (qe *QueryEngine) SubmitMessage(ctx context.Context, input string, opts ...SubmitOption) *QueryResult {
	// Append the user's new input as a message to the conversation history,
	// then pass the full history as Messages (not Input) so that
	// buildInitialHistory uses the complete history including the new turn.
	userMsg := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(input),
		},
	}
	qe.messages = append(qe.messages, userMsg)

	req := conversation.RunRequest{
		SessionID: qe.config.Config.SessionID,
		Messages:  qe.messages,
		CWD:       ".",
	}

	for _, opt := range opts {
		opt(&req)
	}

	result := RunQuery(ctx, qe.config.Runtime, req)

	if result.Err == nil && len(result.Messages) > 0 {
		qe.messages = result.Messages
	}

	return result
}

// Reset clears the conversation history.
func (qe *QueryEngine) Reset() {
	qe.messages = nil
}

// SubmitOption configures a query submission.
type SubmitOption func(*conversation.RunRequest)

// WithCWD sets the current working directory for the query.
func WithCWD(cwd string) SubmitOption {
	return func(req *conversation.RunRequest) {
		req.CWD = cwd
	}
}

// WithSystemPrompt overrides the system prompt for this query.
func WithSystemPrompt(prompt string) SubmitOption {
	return func(req *conversation.RunRequest) {
		req.System = prompt
	}
}

// WithTokenBudget sets the turn token budget for this query.
func WithTokenBudget(budget int) SubmitOption {
	return func(req *conversation.RunRequest) {
		req.TurnTokenBudget = budget
	}
}

// WithPermissionMode sets the permission mode for this query.
func WithPermissionMode(mode string) SubmitOption {
	return func(req *conversation.RunRequest) {
		req.PermissionMode = mode
	}
}
