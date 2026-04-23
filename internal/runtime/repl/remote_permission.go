package repl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

// PermissionBridge converts remote control permission requests into local
// approval events and provides tool stubs for unknown remote tools.
type PermissionBridge struct{}

// NewPermissionBridge creates a permission bridge.
func NewPermissionBridge() *PermissionBridge {
	return &PermissionBridge{}
}

// CreateApprovalEvent builds a synthetic approval event from a remote
// permission request. The returned event carries TypeApprovalRequired so
// the REPL renderer can surface the permission prompt to the user.
func (b *PermissionBridge) CreateApprovalEvent(req *sdk.ControlPermissionRequest, requestID string) event.Event {
	action := "use"
	path := req.ToolName
	if req.BlockedPath != "" {
		path = req.BlockedPath
	}

	message := fmt.Sprintf("Remote tool request: %s", req.ToolName)
	if req.Title != "" {
		message = req.Title
	} else if req.Description != "" {
		message = req.Description
	}
	if len(req.Input) > 0 {
		inputJSON, _ := json.Marshal(req.Input)
		message += fmt.Sprintf("\nInput: %s", string(inputJSON))
	}

	return event.Event{
		Type:      event.TypeApprovalRequired,
		Timestamp: time.Now(),
		Payload: event.ApprovalPayload{
			CallID:   req.ToolUseID,
			ToolName: req.ToolName,
			Path:     path,
			Action:   action,
			Message:  message,
		},
	}
}

// CreateToolStub builds a minimal Tool for a remote tool that is not loaded
// locally. The stub reports that it is not read-only and not concurrency-safe,
// and its Invoke returns an error indicating the tool executes remotely.
func (b *PermissionBridge) CreateToolStub(toolName string) tool.Tool {
	return &remoteToolStub{name: toolName}
}

// remoteToolStub is a minimal tool.Tool implementation for unknown remote tools.
type remoteToolStub struct {
	name string
}

// Name returns the tool name.
func (t *remoteToolStub) Name() string { return t.name }

// Description returns a placeholder description.
func (t *remoteToolStub) Description() string {
	return fmt.Sprintf("Remote tool %s (executes on CCR)", t.name)
}

// InputSchema returns an empty schema.
func (t *remoteToolStub) InputSchema() tool.InputSchema { return tool.InputSchema{} }

// IsReadOnly returns false.
func (t *remoteToolStub) IsReadOnly() bool { return false }

// IsConcurrencySafe returns false.
func (t *remoteToolStub) IsConcurrencySafe() bool { return false }

// Invoke returns an error because remote tools execute on CCR, not locally.
func (t *remoteToolStub) Invoke(_ context.Context, _ tool.Call) (tool.Result, error) {
	return tool.Result{}, fmt.Errorf("remote tool %s executes on CCR, not locally", t.name)
}
