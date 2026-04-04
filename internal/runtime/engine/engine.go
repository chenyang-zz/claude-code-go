package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Engine executes one conversation request and produces runtime events.
type Engine interface {
	Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error)
}

// ToolExecutor resolves one normalized tool call for the runtime loop.
type ToolExecutor interface {
	Execute(ctx context.Context, call coretool.Call) (coretool.Result, error)
}

// Runtime is the minimum single-turn text engine used by batch-07.
type Runtime struct {
	// Client sends the provider request and returns a model event stream.
	Client model.Client
	// DefaultModel is used when the caller does not override the model.
	DefaultModel string
	// ToolCatalog stores the provider-facing tool declarations attached to each request by default.
	ToolCatalog []model.ToolDefinition
	// Executor runs normalized tool invocations when the model emits tool_use blocks.
	Executor ToolExecutor
	// ApprovalService resolves runtime approval prompts for guarded tool operations.
	ApprovalService approval.Service
	// MaxToolIterations caps the number of tool-result feedback loops per runtime turn.
	MaxToolIterations int
}

// New builds the minimum single-turn engine.
func New(client model.Client, defaultModel string, executor ToolExecutor, tools ...model.ToolDefinition) *Runtime {
	return &Runtime{
		Client:            client,
		DefaultModel:      defaultModel,
		ToolCatalog:       append([]model.ToolDefinition(nil), tools...),
		Executor:          executor,
		MaxToolIterations: 8,
	}
}

// Run converts a single input turn into a provider stream and maps it back into runtime events.
func (e *Runtime) Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error) {
	history, err := buildInitialHistory(req)
	if err != nil {
		return nil, err
	}

	logger.DebugCF("engine", "starting single-turn run", map[string]any{
		"session_id":    req.SessionID,
		"message_count": len(history.Messages),
		"model":         e.DefaultModel,
	})

	out := make(chan event.Event)
	go func() {
		defer close(out)
		if err := e.runLoop(ctx, req.SessionID, history, out); err != nil {
			out <- event.Event{
				Type:      event.TypeError,
				Timestamp: time.Now(),
				Payload: event.ErrorPayload{
					Message: err.Error(),
				},
			}
		}

		logger.DebugCF("engine", "single-turn run finished", map[string]any{
			"session_id": req.SessionID,
		})
	}()

	return out, nil
}

// buildInitialHistory normalizes either an explicit message list or a raw user input into the first request history.
func buildInitialHistory(req conversation.RunRequest) (conversation.History, error) {
	if len(req.Messages) > 0 {
		history := conversation.History{Messages: make([]message.Message, len(req.Messages))}
		copy(history.Messages, req.Messages)
		return history, nil
	}

	input := strings.TrimSpace(req.Input)
	if input == "" {
		return conversation.History{}, fmt.Errorf("missing user input")
	}

	return conversation.History{
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.TextPart(input),
				},
			},
		},
	}, nil
}

// runLoop executes the minimal serial tool loop until the model returns plain text without new tool_use blocks.
func (e *Runtime) runLoop(ctx context.Context, sessionID string, history conversation.History, out chan<- event.Event) error {
	toolLoops := 0
	for {
		modelStream, err := e.Client.Stream(ctx, model.Request{
			Model:    e.DefaultModel,
			Messages: history.Messages,
			Tools:    e.ToolCatalog,
		})
		if err != nil {
			return err
		}

		assistantMessage, pendingToolUses, err := e.consumeModelStream(modelStream, out)
		if err != nil {
			return err
		}
		if len(assistantMessage.Content) > 0 {
			history.Append(assistantMessage)
		}
		if len(pendingToolUses) == 0 {
			return nil
		}
		if e.Executor == nil {
			return fmt.Errorf("tool loop requested but executor is not configured")
		}
		if toolLoops >= e.maxToolIterations() {
			return fmt.Errorf("tool loop exceeded max iterations (%d)", e.maxToolIterations())
		}

		toolLoops++
		logger.DebugCF("engine", "executing tool loop iteration", map[string]any{
			"session_id": sessionID,
			"tool_count": len(pendingToolUses),
			"iteration":  toolLoops,
		})

		history.Append(e.executeToolUses(ctx, pendingToolUses, out))
	}
}

// consumeModelStream aggregates one provider response into an assistant message plus any completed tool_use blocks.
func (e *Runtime) consumeModelStream(modelStream model.Stream, out chan<- event.Event) (message.Message, []model.ToolUse, error) {
	assistant := message.Message{Role: message.RoleAssistant}
	var toolUses []model.ToolUse

	for item := range modelStream {
		switch item.Type {
		case model.EventTypeTextDelta:
			assistant.Content = append(assistant.Content, message.TextPart(item.Text))
			out <- event.Event{
				Type:      event.TypeMessageDelta,
				Timestamp: time.Now(),
				Payload: event.MessageDeltaPayload{
					Text: item.Text,
				},
			}
		case model.EventTypeError:
			return message.Message{}, nil, errors.New(item.Error)
		case model.EventTypeToolUse:
			if item.ToolUse == nil {
				return message.Message{}, nil, fmt.Errorf("tool use event missing payload")
			}
			toolUses = append(toolUses, *item.ToolUse)
			assistant.Content = append(assistant.Content, message.ToolUsePart(item.ToolUse.ID, item.ToolUse.Name, item.ToolUse.Input))
			out <- event.Event{
				Type:      event.TypeToolCallStarted,
				Timestamp: time.Now(),
				Payload: event.ToolCallPayload{
					ID:    item.ToolUse.ID,
					Name:  item.ToolUse.Name,
					Input: item.ToolUse.Input,
				},
			}
		}
	}

	return assistant, toolUses, nil
}

// executeToolUses resolves one serial batch of tool calls and converts the results into a single tool_result message.
func (e *Runtime) executeToolUses(ctx context.Context, toolUses []model.ToolUse, out chan<- event.Event) message.Message {
	resultMessage := message.Message{Role: message.RoleUser}

	for _, toolUse := range toolUses {
		call := coretool.Call{
			ID:     toolUse.ID,
			Name:   toolUse.Name,
			Input:  toolUse.Input,
			Source: "model",
		}
		result, invokeErr := e.executeToolUse(ctx, call, out)

		content, isError := renderToolResult(result, invokeErr)
		resultMessage.Content = append(resultMessage.Content, message.ToolResultPart(toolUse.ID, content, isError))
		out <- event.Event{
			Type:      event.TypeToolCallFinished,
			Timestamp: time.Now(),
			Payload: event.ToolResultPayload{
				ID:      toolUse.ID,
				Name:    toolUse.Name,
				Output:  content,
				IsError: isError,
			},
		}
	}

	return resultMessage
}

// executeToolUse resolves one tool call and branches into the approval flow when the tool is blocked by a permission ask.
func (e *Runtime) executeToolUse(ctx context.Context, call coretool.Call, out chan<- event.Event) (coretool.Result, error) {
	result, invokeErr := e.Executor.Execute(ctx, call)
	if invokeErr == nil {
		return result, nil
	}

	var permissionErr *corepermission.PermissionError
	if !errors.As(invokeErr, &permissionErr) || permissionErr.Decision != corepermission.DecisionAsk || e.ApprovalService == nil {
		return result, invokeErr
	}

	out <- event.Event{
		Type:      event.TypeApprovalRequired,
		Timestamp: time.Now(),
		Payload: event.ApprovalPayload{
			CallID:   call.ID,
			ToolName: call.Name,
			Path:     permissionErr.Path,
			Action:   string(permissionErr.Access),
			Message:  permissionErr.Message,
		},
	}

	decision, err := e.ApprovalService.Decide(ctx, approval.Request{
		CallID:   call.ID,
		ToolName: call.Name,
		Path:     permissionErr.Path,
		Action:   string(permissionErr.Access),
		Message:  permissionErr.Message,
	})
	if err != nil {
		return coretool.Result{}, err
	}
	if !decision.Approved {
		if strings.TrimSpace(decision.Reason) == "" {
			decision.Reason = fmt.Sprintf("Permission to %s %s was not granted.", permissionErr.Access, permissionErr.Path)
		}
		return coretool.Result{Error: decision.Reason}, nil
	}

	retryCtx := corepermission.WithFilesystemGrant(ctx, corepermission.FilesystemRequest{
		ToolName:   call.Name,
		Path:       permissionErr.Path,
		WorkingDir: call.Context.WorkingDir,
		Access:     permissionErr.Access,
	})
	return e.Executor.Execute(retryCtx, call)
}

// renderToolResult normalizes executor success and failure paths into the minimal tool_result payload understood by the model.
func renderToolResult(result coretool.Result, invokeErr error) (string, bool) {
	if invokeErr != nil {
		if strings.TrimSpace(result.Error) != "" {
			return result.Error, true
		}
		return invokeErr.Error(), true
	}
	if strings.TrimSpace(result.Error) != "" {
		return result.Error, true
	}
	return result.Output, false
}

// maxToolIterations returns the configured loop cap, falling back to the default minimum when unset.
func (e *Runtime) maxToolIterations() int {
	if e == nil || e.MaxToolIterations <= 0 {
		return 8
	}
	return e.MaxToolIterations
}

// DescribeTools converts a tool registry into provider-facing tool definitions.
func DescribeTools(registry coretool.Registry) []model.ToolDefinition {
	if registry == nil {
		return nil
	}

	registered := registry.List()
	descriptions := make([]model.ToolDefinition, 0, len(registered))
	for _, item := range registered {
		if item == nil {
			continue
		}
		descriptions = append(descriptions, model.ToolDefinition{
			Name:        item.Name(),
			Description: item.Description(),
			InputSchema: item.InputSchema().JSONSchema(),
		})
	}
	return descriptions
}
