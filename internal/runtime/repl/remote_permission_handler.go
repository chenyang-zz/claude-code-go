package repl

import (
	"context"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/platform/remote"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

// RemotePermissionHandler wires remote permission requests into the local
// approval service and sends responses back through the session manager.
type RemotePermissionHandler struct {
	bridge          *PermissionBridge
	approvalService approval.Service
}

// NewRemotePermissionHandler creates a handler with the given approval service.
func NewRemotePermissionHandler(approvalService approval.Service) *RemotePermissionHandler {
	return &RemotePermissionHandler{
		bridge:          NewPermissionBridge(),
		approvalService: approvalService,
	}
}

// HandlePermissionRequest processes one remote permission request through the
// local approval service. It renders the approval prompt and sends the result
// back via the session manager. This method blocks until the user decides; it
// should be called in its own goroutine.
func (h *RemotePermissionHandler) HandlePermissionRequest(ctx context.Context, sm *remote.SessionManager, req *sdk.ControlPermissionRequest, requestID string) {
	evt := h.bridge.CreateApprovalEvent(req, requestID)

	payload := evt.Payload.(event.ApprovalPayload)
	approvalReq := approval.Request{
		CallID:   payload.CallID,
		ToolName: payload.ToolName,
		Path:     payload.Path,
		Action:   payload.Action,
		Message:  payload.Message,
	}

	resp, err := h.approvalService.Decide(ctx, approvalReq)
	if err != nil {
		logger.WarnCF("repl", "approval service error for remote permission", map[string]any{
			"request_id": requestID,
			"tool_name":  req.ToolName,
			"error":      err.Error(),
		})
		_ = sm.RespondToPermissionRequest(requestID, sdk.PermissionResponse{
			Behavior: "deny",
			Message:  fmt.Sprintf("approval error: %v", err),
		})
		return
	}

	if resp.Approved {
		_ = sm.RespondToPermissionRequest(requestID, sdk.PermissionResponse{
			Behavior:     "allow",
			UpdatedInput: req.Input,
		})
	} else {
		_ = sm.RespondToPermissionRequest(requestID, sdk.PermissionResponse{
			Behavior: "deny",
			Message:  resp.Reason,
		})
	}
}
