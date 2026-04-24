package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const defaultResponsesPath = "/v1/responses"

// ResponsesClient implements model.Client for the OpenAI Responses API.
type ResponsesClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewResponsesClient builds a client targeting the OpenAI Responses API.
func NewResponsesClient(cfg Config) *ResponsesClient {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &ResponsesClient{
		apiKey:     cfg.APIKey,
		baseURL:    strings.TrimRight(resolveBaseURL("", cfg.BaseURL), "/"),
		httpClient: httpClient,
	}
}

// Stream opens a Responses API streaming request and converts SSE events into
// model events.
func (c *ResponsesClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("missing API key for responses provider")
	}

	body, err := json.Marshal(buildResponsesRequest(req))
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+defaultResponsesPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build responses request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "text/event-stream")
	httpReq.Header.Set("authorization", "Bearer "+c.apiKey)

	logger.DebugCF("responses_client", "starting responses api stream", map[string]any{
		"model":         req.Model,
		"message_count": len(req.Messages),
		"base_url":      c.baseURL,
	})

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute responses request: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		payload, _ := io.ReadAll(resp.Body)
		return nil, ParseAPIError(resp, payload)
	}

	out := make(chan model.Event)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var state responsesStreamState
		var dataLines []string

		flush := func() {
			if len(dataLines) == 0 {
				return
			}
			c.handleEvent(strings.Join(dataLines, "\n"), &state, out)
			dataLines = dataLines[:0]
		}

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				flush()
				continue
			}
			if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		flush()

		if err := scanner.Err(); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("read responses stream: %v", err),
			}
			return
		}

		c.emitDone(&state, out)
	}()

	return out, nil
}

// responsesStreamState accumulates metadata across one Responses API stream.
type responsesStreamState struct {
	finishReason     string
	promptTokens     int
	completionTokens int
	textBuffer       strings.Builder
	callID           string
	callName         string
	callArgs         strings.Builder
	inFunctionCall   bool
	responseID       string
	status           string
	incompleteReason string
}

// handleEvent parses one SSE data block and emits model events.
func (c *ResponsesClient) handleEvent(data string, state *responsesStreamState, out chan<- model.Event) {
	if data == "" || data == "[DONE]" {
		return
	}

	var event responsesStreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		out <- model.Event{
			Type:  model.EventTypeError,
			Error: fmt.Sprintf("parse responses stream event: %v", err),
		}
		return
	}

	switch event.Type {
	case responsesEventOutputItemAdded:
		if event.Item != nil && event.Item.Type == "function_call" {
			state.inFunctionCall = true
			state.callID = event.Item.CallID
			state.callName = event.Item.Name
			state.callArgs.Reset()
		}
		if event.Item != nil && event.Item.Type == "message" {
			state.textBuffer.Reset()
		}

	case responsesEventOutputTextDelta:
		if event.Delta != "" {
			state.textBuffer.WriteString(event.Delta)
			out <- model.Event{
				Type: model.EventTypeTextDelta,
				Text: event.Delta,
			}
		}

	case responsesEventFunctionCallArgsDelta:
		if event.ArgumentsDelta != "" {
			state.callArgs.WriteString(event.ArgumentsDelta)
		}

	case responsesEventOutputItemDone:
		if event.Item != nil {
			switch event.Item.Type {
			case "message":
				// Text already emitted as deltas; nothing more to do.
			case "function_call":
				c.emitToolUse(state, out)
				state.inFunctionCall = false
			}
		}

	case responsesEventDone:
		if event.Response != nil && event.Response.Usage != nil {
			state.promptTokens = event.Response.Usage.InputTokens
			state.completionTokens = event.Response.Usage.OutputTokens
		}

	case responsesEventCompleted:
		if event.Response != nil {
			state.responseID = event.Response.ID
			state.status = event.Response.Status
			if event.Response.IncompleteDetails != nil {
				state.incompleteReason = event.Response.IncompleteDetails.Reason
			}
			if event.Response.Usage != nil {
				state.promptTokens = event.Response.Usage.InputTokens
				state.completionTokens = event.Response.Usage.OutputTokens
			}
		}

	case responsesEventIncomplete:
		if event.Response != nil {
			state.status = event.Response.Status
			if event.Response.IncompleteDetails != nil {
				state.incompleteReason = event.Response.IncompleteDetails.Reason
			}
		}

	case responsesEventFailed:
		if event.Response != nil && event.Response.Error != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("responses api error: %s", event.Response.Error.Message),
			}
		}
	}
}

// emitToolUse flushes a completed function call into a model event.
func (c *ResponsesClient) emitToolUse(state *responsesStreamState, out chan<- model.Event) {
	input := make(map[string]any)
	args := strings.TrimSpace(state.callArgs.String())
	if args != "" {
		if err := json.Unmarshal([]byte(args), &input); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse responses function arguments: %v", err),
			}
			return
		}
	}
	out <- model.Event{
		Type: model.EventTypeToolUse,
		ToolUse: &model.ToolUse{
			ID:    state.callID,
			Name:  state.callName,
			Input: input,
		},
	}
}

// emitDone sends the terminal done event with accumulated metadata.
func (c *ResponsesClient) emitDone(state *responsesStreamState, out chan<- model.Event) {
	doneEvent := model.Event{
		Type:       model.EventTypeDone,
		ResponseID: state.responseID,
	}
	if state.promptTokens > 0 || state.completionTokens > 0 {
		doneEvent.Usage = &model.Usage{
			InputTokens:  state.promptTokens,
			OutputTokens: state.completionTokens,
		}
	}
	// Propagate stop reason based on the final response status.
	switch state.status {
	case "incomplete":
		doneEvent.StopReason = model.StopReasonMaxTokens
	case "failed":
		// The error has already been emitted via responsesEventFailed.
	default:
		if state.inFunctionCall {
			doneEvent.StopReason = model.StopReasonToolUse
		} else {
			doneEvent.StopReason = model.StopReasonEndTurn
		}
	}
	out <- doneEvent
}
