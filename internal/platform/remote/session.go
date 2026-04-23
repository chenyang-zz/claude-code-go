package remote

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

// MessageSender sends raw bytes over the remote transport.
type MessageSender interface {
	Send(data []byte) error
}

// SessionCallbacks receives events from the remote session manager.
type SessionCallbacks struct {
	// OnSDKMessage is called for each SDK message received from the remote session.
	OnSDKMessage func(data []byte)
	// OnPermissionRequest is called when a permission request arrives from CCR.
	OnPermissionRequest func(req *sdk.ControlPermissionRequest, requestID string)
	// OnPermissionCancelled is called when the server cancels a pending permission request.
	OnPermissionCancelled func(requestID string, toolUseID string)
}

// SessionManager coordinates remote event classification, permission request
// tracking, control message responses, and subagent state for one remote session.
type SessionManager struct {
	config    coreconfig.RemoteSessionConfig
	sender    MessageSender
	callbacks SessionCallbacks
	mu        sync.Mutex
	pending   map[string]*sdk.ControlPermissionRequest
	// Subagents tracks known subagent states observed from the remote session.
	Subagents *SubagentRegistry
}

// NewSessionManager constructs one session manager with the given config,
// sender, and callbacks.
func NewSessionManager(config coreconfig.RemoteSessionConfig, sender MessageSender, callbacks SessionCallbacks) *SessionManager {
	return &SessionManager{
		config:    config,
		sender:    sender,
		callbacks: callbacks,
		pending:   make(map[string]*sdk.ControlPermissionRequest),
		Subagents: NewSubagentRegistry(),
	}
}

// HandleEvent classifies and routes one remote event.
func (s *SessionManager) HandleEvent(evt Event) {
	if s == nil {
		return
	}

	category, err := ClassifyEvent(evt.Data)
	if err != nil {
		logger.WarnCF("remote_session", "failed to classify event", map[string]any{
			"error": err.Error(),
		})
		return
	}

	switch category {
	case CategoryControlRequest:
		s.handleControlRequest(evt.Data)
	case CategoryControlCancelRequest:
		s.handleControlCancelRequest(evt.Data)
	case CategoryControlResponse:
		logger.DebugCF("remote_session", "received control response", nil)
	case CategorySDKMessage:
		s.trackSubagentFromEvent(evt.Data)
		if s.callbacks.OnSDKMessage != nil {
			s.callbacks.OnSDKMessage(evt.Data)
		}
	}
}

// RespondToPermissionRequest sends a control response for a pending permission request.
func (s *SessionManager) RespondToPermissionRequest(requestID string, result sdk.PermissionResponse) error {
	if s == nil {
		return fmt.Errorf("session manager is nil")
	}

	s.mu.Lock()
	_, ok := s.pending[requestID]
	delete(s.pending, requestID)
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("no pending permission request with ID: %s", requestID)
	}

	responseData := map[string]any{
		"behavior": result.Behavior,
	}
	if result.Behavior == "allow" {
		responseData["updatedInput"] = result.UpdatedInput
	} else {
		responseData["message"] = result.Message
	}

	resp := sdk.ControlResponse{
		Type: "control_response",
		Response: sdk.ControlResponseInner{
			Subtype:   "success",
			RequestID: requestID,
			Response:  responseData,
		},
	}

	return s.sendControlResponse(&resp)
}

// SendInterrupt sends an interrupt control request to the remote session.
func (s *SessionManager) SendInterrupt() error {
	if s == nil {
		return fmt.Errorf("session manager is nil")
	}

	inner, err := json.Marshal(sdk.ControlInterruptRequest{Subtype: "interrupt"})
	if err != nil {
		return fmt.Errorf("marshal interrupt inner: %w", err)
	}

	req := sdk.ControlRequest{
		Type:      "control_request",
		RequestID: uuid.NewString(),
		Request:   inner,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal interrupt request: %w", err)
	}

	if err := s.sender.Send(data); err != nil {
		return fmt.Errorf("send interrupt request: %w", err)
	}
	return nil
}

// Disconnect clears all pending permission state and subagent tracking.
func (s *SessionManager) Disconnect() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.pending = make(map[string]*sdk.ControlPermissionRequest)
	s.mu.Unlock()
	if s.Subagents != nil {
		s.Subagents.Clear()
	}
}

// SubagentCount returns the number of known subagents. Implements RemoteSubagentStateProvider.
func (s *SessionManager) SubagentCount() int {
	if s == nil || s.Subagents == nil {
		return 0
	}
	return s.Subagents.Count()
}

// SubagentList returns a snapshot of all known subagent states. Implements RemoteSubagentStateProvider.
func (s *SessionManager) SubagentList() []SubagentStateView {
	if s == nil || s.Subagents == nil {
		return nil
	}
	states := s.Subagents.List()
	result := make([]SubagentStateView, len(states))
	for i, st := range states {
		result[i] = SubagentStateView{
			AgentID:    st.AgentID,
			AgentType:  st.AgentType,
			Status:     st.Status,
			EventCount: st.EventCount,
		}
	}
	return result
}

// PendingCount returns the number of pending permission requests.
func (s *SessionManager) PendingCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

func (s *SessionManager) handleControlRequest(data []byte) {
	var req sdk.ControlRequest
	if err := json.Unmarshal(data, &req); err != nil {
		logger.WarnCF("remote_session", "failed to unmarshal control request", map[string]any{
			"error": err.Error(),
		})
		return
	}

	var inner struct {
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal(req.Request, &inner); err != nil {
		logger.WarnCF("remote_session", "failed to unmarshal control request inner", map[string]any{
			"error": err.Error(),
		})
		return
	}

	switch inner.Subtype {
	case "can_use_tool":
		var permReq sdk.ControlPermissionRequest
		if err := json.Unmarshal(req.Request, &permReq); err != nil {
			logger.WarnCF("remote_session", "failed to unmarshal permission request", map[string]any{
				"error": err.Error(),
			})
			return
		}
		s.mu.Lock()
		s.pending[req.RequestID] = &permReq
		s.mu.Unlock()
		if s.callbacks.OnPermissionRequest != nil {
			s.callbacks.OnPermissionRequest(&permReq, req.RequestID)
		}
	default:
		logger.DebugCF("remote_session", "unsupported control request subtype", map[string]any{
			"subtype": inner.Subtype,
		})
		resp := sdk.ControlResponse{
			Type: "control_response",
			Response: sdk.ControlResponseInner{
				Subtype:   "error",
				RequestID: req.RequestID,
				Error:     fmt.Sprintf("unsupported control request subtype: %s", inner.Subtype),
			},
		}
		_ = s.sendControlResponse(&resp)
	}
}

func (s *SessionManager) handleControlCancelRequest(data []byte) {
	var req sdk.ControlCancelRequest
	if err := json.Unmarshal(data, &req); err != nil {
		logger.WarnCF("remote_session", "failed to unmarshal control cancel request", map[string]any{
			"error": err.Error(),
		})
		return
	}

	s.mu.Lock()
	pendingReq, ok := s.pending[req.RequestID]
	delete(s.pending, req.RequestID)
	s.mu.Unlock()

	var toolUseID string
	if ok && pendingReq != nil {
		toolUseID = pendingReq.ToolUseID
	}

	if s.callbacks.OnPermissionCancelled != nil {
		s.callbacks.OnPermissionCancelled(req.RequestID, toolUseID)
	}
}

// trackSubagentFromEvent inspects the raw JSON payload for an agent_id field
// and updates the subagent registry when one is found.
func (s *SessionManager) trackSubagentFromEvent(data []byte) {
	if s == nil || s.Subagents == nil {
		return
	}
	var envelope struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return
	}
	if envelope.AgentID == "" {
		return
	}
	s.Subagents.UpdateFromEvent(InternalEvent{AgentID: envelope.AgentID})
}

func (s *SessionManager) sendControlResponse(resp *sdk.ControlResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		logger.WarnCF("remote_session", "failed to marshal control response", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("marshal control response: %w", err)
	}
	if err := s.sender.Send(data); err != nil {
		logger.WarnCF("remote_session", "failed to send control response", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("send control response: %w", err)
	}
	return nil
}
