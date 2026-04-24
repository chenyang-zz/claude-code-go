package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// buildResponsesRequest converts a normalized engine request into an OpenAI
// Responses API payload.
func buildResponsesRequest(req model.Request) responsesRequest {
	r := responsesRequest{
		Model:           req.Model,
		Input:           mapMessagesToResponsesInput(req.System, req.Messages),
		Tools:           mapToolsToResponses(req.Tools),
		Stream:          true,
		MaxOutputTokens: req.MaxOutputTokens,
	}

	// Map advanced parameters.
	if req.Instructions != nil {
		r.Instructions = *req.Instructions
	}
	if req.PreviousResponseID != nil {
		r.PreviousResponseID = *req.PreviousResponseID
	}
	if req.Store != nil {
		r.Store = req.Store
	}
	if req.ReasoningEffort != nil {
		r.Reasoning = &responsesReasoning{Effort: *req.ReasoningEffort}
	}
	if req.Temperature != nil {
		r.Temperature = req.Temperature
	}
	if req.TopP != nil {
		r.TopP = req.TopP
	}
	if req.ToolChoice != nil {
		r.ToolChoice = mapToolChoice(*req.ToolChoice)
	}
	if len(req.Metadata) > 0 {
		r.Metadata = req.Metadata
	}
	if req.User != nil {
		r.User = *req.User
	}

	return r
}

// mapToolChoice converts the internal tool choice string to the Responses
// API tool_choice union type.
func mapToolChoice(choice string) *responsesToolChoice {
	switch choice {
	case "auto", "none", "required":
		return &responsesToolChoice{Mode: choice}
	default:
		// Support "function:<name>" syntax for forcing a specific tool.
		if name, ok := strings.CutPrefix(choice, "function:"); ok {
			return &responsesToolChoice{FunctionName: name}
		}
		// Fallback: treat as a mode string.
		return &responsesToolChoice{Mode: choice}
	}
}

// mapMessagesToResponsesInput converts the internal message history into the
// Responses API input array.
func mapMessagesToResponsesInput(system string, history []message.Message) []responsesInputItem {
	out := make([]responsesInputItem, 0, len(history)+1)

	if strings.TrimSpace(system) != "" {
		out = append(out, responsesInputItem{
			Role:    "developer",
			Content: system,
		})
	}

	for _, item := range history {
		switch item.Role {
		case message.RoleUser:
			text := collectText(item.Content)
			if text != "" {
				out = append(out, responsesInputItem{
					Role:    string(message.RoleUser),
					Content: text,
				})
			}
			for _, part := range item.Content {
				if part.Type != "tool_result" {
					continue
				}
				out = append(out, responsesInputItem{
					Role:    string(message.RoleUser),
					Content: fmt.Sprintf("<tool_result tool_use_id=\"%s\">%s</tool_result>", part.ToolUseID, part.Text),
				})
			}

		case message.RoleAssistant:
			assistantText := collectText(item.Content)
			if assistantText != "" {
				out = append(out, responsesInputItem{
					Role:    string(message.RoleAssistant),
					Content: assistantText,
				})
			}
			// Tool calls made by the assistant are represented as separate
			// function_call output items in Responses API, so we emit them
			// as user-role items that reference the call.
			for _, part := range item.Content {
				if part.Type != "tool_use" {
					continue
				}
				args, _ := json.Marshal(part.ToolInput)
				out = append(out, responsesInputItem{
					Role:    string(message.RoleUser),
					Content: fmt.Sprintf("<function_call call_id=\"%s\" name=\"%s\">%s</function_call>", part.ToolUseID, part.ToolName, string(args)),
				})
			}
		}
	}

	return out
}

// mapToolsToResponses converts internal tool definitions into the Responses
// API tool envelope shape.
func mapToolsToResponses(tools []model.ToolDefinition) []responsesTool {
	if len(tools) == 0 {
		return nil
	}

	out := make([]responsesTool, 0, len(tools))
	for _, item := range tools {
		out = append(out, responsesTool{
			Type: "function",
			Function: toolSpecBody{
				Name:        item.Name,
				Description: item.Description,
				Parameters:  item.InputSchema,
			},
		})
	}
	return out
}

// parseResponsesOutput walks the output items from a Responses API response
// and emits normalized model events.
func parseResponsesOutput(items []responsesOutputItem) ([]model.Event, error) {
	var events []model.Event

	for _, item := range items {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" && part.Text != "" {
					events = append(events, model.Event{
						Type: model.EventTypeTextDelta,
						Text: part.Text,
					})
				}
			}

		case "function_call":
			input := make(map[string]any)
			args := strings.TrimSpace(item.Arguments)
			if args != "" {
				if err := json.Unmarshal([]byte(args), &input); err != nil {
					return nil, fmt.Errorf("parse responses function call arguments: %w", err)
				}
			}
			events = append(events, model.Event{
				Type: model.EventTypeToolUse,
				ToolUse: &model.ToolUse{
					ID:    item.CallID,
					Name:  item.Name,
					Input: input,
				},
			})
		}
	}

	return events, nil
}
