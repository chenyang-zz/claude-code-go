package repl

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/internal/platform/remote"
	"github.com/sheepzhao/claude-code-go/internal/ui/console"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

// RemoteEventBridge converts remote transport events to core event.Events and renders them.
type RemoteEventBridge struct {
	Renderer console.EventRenderer
}

// OnEvent returns a callback that bridges remote events to the renderer.
func (b *RemoteEventBridge) OnEvent() func(remote.Event) {
	return func(evt remote.Event) {
		coreEvt, err := ConvertRemoteEvent(evt)
		if err != nil {
			logger.WarnCF("repl", "failed to convert remote event", map[string]any{
				"transport": string(evt.Transport),
				"type":      evt.Type,
				"error":     err.Error(),
			})
			_ = b.Renderer.RenderEvent(event.Event{
				Type:      event.TypeError,
				Timestamp: time.Now(),
				Payload:   event.ErrorPayload{Message: fmt.Sprintf("remote event bridge error: %v", err)},
			})
			return
		}
		if coreEvt == nil {
			return
		}
		if err := b.Renderer.RenderEvent(*coreEvt); err != nil {
			logger.WarnCF("repl", "failed to render remote event", map[string]any{
				"event_type": string(coreEvt.Type),
				"error":      err.Error(),
			})
		}
	}
}

// ConvertRemoteEvent parses one raw remote transport event into a core event.Event.
// Events that have no visible rendering counterpart return nil.
func ConvertRemoteEvent(evt remote.Event) (*event.Event, error) {
	var typeOnly struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal(evt.Data, &typeOnly); err != nil {
		return nil, fmt.Errorf("unmarshal remote event type: %w", err)
	}

	now := time.Now()

	switch typeOnly.Type {
	case "stream_event":
		return convertStreamEvent(evt.Data, now)
	case "result":
		return convertResultEvent(evt.Data, now)
	case "tool_progress":
		return convertToolProgressEvent(evt.Data, now)
	case "system":
		return convertSystemEvent(evt.Data, typeOnly.Subtype, now)
	case "assistant", "user", "auth_status", "tool_use_summary", "rate_limit_event":
		// These types are handled locally or have no REPL rendering counterpart.
		return nil, nil
	default:
		logger.DebugCF("repl", "ignoring unknown remote event type", map[string]any{
			"type": typeOnly.Type,
		})
		return nil, nil
	}
}

// convertStreamEvent handles SDK stream_event messages (text_delta, etc.).
func convertStreamEvent(data []byte, now time.Time) (*event.Event, error) {
	var se sdk.StreamEvent
	if err := json.Unmarshal(data, &se); err != nil {
		return nil, fmt.Errorf("unmarshal stream_event: %w", err)
	}

	eventMap, ok := se.Event.(map[string]any)
	if !ok {
		return nil, nil
	}

	eventType, _ := eventMap["type"].(string)
	switch eventType {
	case "text_delta":
		text, _ := eventMap["text"].(string)
		return &event.Event{
			Type:      event.TypeMessageDelta,
			Timestamp: now,
			Payload:   event.MessageDeltaPayload{Text: text},
		}, nil
	default:
		// Other stream_event types (e.g. input_json_delta) are not rendered in REPL.
		logger.DebugCF("repl", "ignoring unhandled stream_event subtype", map[string]any{
			"event_type": eventType,
		})
		return nil, nil
	}
}

// convertResultEvent handles SDK result messages (success/error).
func convertResultEvent(data []byte, now time.Time) (*event.Event, error) {
	var result sdk.Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	if result.IsError {
		msg := "Unknown error"
		if len(result.Errors) > 0 {
			msg = result.Errors[0]
		}
		return &event.Event{
			Type:      event.TypeError,
			Timestamp: now,
			Payload:   event.ErrorPayload{Message: msg},
		}, nil
	}

	// Success result signals conversation completion for the render chain.
	usage := extractUsage(result.Usage)
	stopReason := "end_turn"
	if result.StopReason != nil && *result.StopReason != "" {
		stopReason = *result.StopReason
	}
	return &event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: now,
		Payload: event.ConversationDonePayload{
			Usage:      usage,
			StopReason: stopReason,
		},
	}, nil
}

// convertToolProgressEvent handles SDK tool_progress messages.
func convertToolProgressEvent(data []byte, now time.Time) (*event.Event, error) {
	var tp sdk.ToolProgress
	if err := json.Unmarshal(data, &tp); err != nil {
		return nil, fmt.Errorf("unmarshal tool_progress: %w", err)
	}

	var parentID string
	if tp.ParentToolUseID != nil {
		parentID = *tp.ParentToolUseID
	}

	return &event.Event{
		Type:      event.TypeProgress,
		Timestamp: now,
		Payload: event.ProgressPayload{
			ToolUseID:       tp.ToolUseID,
			ParentToolUseID: parentID,
			Data: map[string]any{
				"tool_name":        tp.ToolName,
				"elapsed_time_sec": tp.ElapsedTimeSec,
			},
		},
	}, nil
}

// convertSystemEvent handles SDK system messages (init, status, compact_boundary).
func convertSystemEvent(data []byte, subtype string, now time.Time) (*event.Event, error) {
	_ = data // reserved for future subtype parsing
	switch subtype {
	case "compact_boundary":
		return &event.Event{
			Type:      event.TypeCompactDone,
			Timestamp: now,
			Payload:   event.CompactDonePayload{},
		}, nil
	case "init", "status":
		// Init and status messages are informational; not rendered as standalone events.
		return nil, nil
	default:
		logger.DebugCF("repl", "ignoring system message subtype", map[string]any{
			"subtype": subtype,
		})
		return nil, nil
	}
}

// extractUsage attempts to coerce the SDK usage field into model.Usage.
func extractUsage(usage any) model.Usage {
	if usage == nil {
		return model.Usage{}
	}

	// Fast path: if the SDK already deserialized into model.Usage-compatible JSON.
	raw, _ := json.Marshal(usage)
	var u model.Usage
	if err := json.Unmarshal(raw, &u); err == nil {
		return u
	}
	return model.Usage{}
}
