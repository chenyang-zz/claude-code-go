package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const defaultBaseURL = "https://api.anthropic.com"

// Config carries the minimum Anthropic client configuration needed by batch-07.
type Config struct {
	// APIKey carries the Anthropic credential.
	APIKey string
	// AuthToken carries the Anthropic bearer token when first-party account auth is used.
	AuthToken string
	// BaseURL optionally overrides the default Anthropic API host.
	BaseURL string
	// HTTPClient allows tests to inject a local transport.
	HTTPClient *http.Client
}

// Client implements the minimum Anthropic SSE text stream client used by the runtime engine.
type Client struct {
	// apiKey stores the request credential.
	apiKey string
	// authToken stores the bearer token credential when first-party account auth is used.
	authToken string
	// baseURL stores the API host root.
	baseURL string
	// httpClient performs HTTP requests.
	httpClient *http.Client
}

type messagesRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   any            `json:"content,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type streamContentBlock struct {
	blockType string
	toolID    string
	toolName  string
	inputJSON strings.Builder
}

type contentBlockStartEnvelope struct {
	Index        int `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
}

type contentBlockDeltaEnvelope struct {
	Index int `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
}

type contentBlockStopEnvelope struct {
	Index int `json:"index"`
}

type errorEnvelope struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// NewClient builds a minimal Anthropic streaming client.
func NewClient(cfg Config) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		apiKey:     cfg.APIKey,
		authToken:  cfg.AuthToken,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

// Stream opens one Anthropic streaming request and converts SSE payloads into model events.
func (c *Client) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	if c.apiKey == "" && c.authToken == "" {
		return nil, fmt.Errorf("missing Anthropic auth credential")
	}

	body, err := json.Marshal(messagesRequest{
		Model:     req.Model,
		MaxTokens: 1024,
		Stream:    true,
		Messages:  mapMessages(req.Messages),
		Tools:     mapTools(req.Tools),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build anthropic request: %w", err)
	}

	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "text/event-stream")
	if c.apiKey != "" {
		httpReq.Header.Set("x-api-key", c.apiKey)
	}
	if c.authToken != "" {
		httpReq.Header.Set("authorization", "Bearer "+c.authToken)
	}
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	logger.DebugCF("anthropic_client", "starting anthropic stream", map[string]any{
		"model":         req.Model,
		"message_count": len(req.Messages),
		"base_url":      c.baseURL,
	})

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute anthropic request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic api error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	out := make(chan model.Event)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var eventName string
		var dataLines []string
		contentBlocks := make(map[int]*streamContentBlock)

		flush := func() {
			if len(dataLines) == 0 {
				eventName = ""
				return
			}

			c.handleSSEEvent(eventName, strings.Join(dataLines, "\n"), contentBlocks, out)
			eventName = ""
			dataLines = dataLines[:0]
		}

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				flush()
				continue
			}

			switch {
			case strings.HasPrefix(line, "event:"):
				eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		flush()

		if err := scanner.Err(); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("read anthropic stream: %v", err),
			}
			return
		}

		out <- model.Event{Type: model.EventTypeDone}
	}()

	return out, nil
}

// handleSSEEvent maps one Anthropic SSE event into the shared model stream format.
func (c *Client) handleSSEEvent(eventName, data string, contentBlocks map[int]*streamContentBlock, out chan<- model.Event) {
	if data == "" || data == "[DONE]" {
		return
	}

	switch eventName {
	case "content_block_start":
		var payload contentBlockStartEnvelope
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse anthropic content block start: %v", err),
			}
			return
		}
		if payload.ContentBlock.Type == "tool_use" {
			contentBlocks[payload.Index] = &streamContentBlock{
				blockType: payload.ContentBlock.Type,
				toolID:    payload.ContentBlock.ID,
				toolName:  payload.ContentBlock.Name,
			}
		}
	case "content_block_delta":
		var payload contentBlockDeltaEnvelope
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse anthropic text delta: %v", err),
			}
			return
		}
		if payload.Delta.Type == "text_delta" && payload.Delta.Text != "" {
			out <- model.Event{
				Type: model.EventTypeTextDelta,
				Text: payload.Delta.Text,
			}
		}
		if payload.Delta.Type == "input_json_delta" {
			block, ok := contentBlocks[payload.Index]
			if !ok || block.blockType != "tool_use" {
				out <- model.Event{
					Type:  model.EventTypeError,
					Error: "received tool input delta for unknown content block",
				}
				return
			}
			block.inputJSON.WriteString(payload.Delta.PartialJSON)
		}
	case "content_block_stop":
		var payload contentBlockStopEnvelope
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse anthropic content block stop: %v", err),
			}
			return
		}

		block, ok := contentBlocks[payload.Index]
		if !ok || block.blockType != "tool_use" {
			return
		}
		defer delete(contentBlocks, payload.Index)

		input, err := parseToolUseInput(block.inputJSON.String())
		if err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse anthropic tool use input: %v", err),
			}
			return
		}

		out <- model.Event{
			Type: model.EventTypeToolUse,
			ToolUse: &model.ToolUse{
				ID:    block.toolID,
				Name:  block.toolName,
				Input: input,
			},
		}
	case "error":
		var payload errorEnvelope
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse anthropic error event: %v", err),
			}
			return
		}
		out <- model.Event{
			Type:  model.EventTypeError,
			Error: payload.Error.Message,
		}
	}
}

// parseToolUseInput decodes the final accumulated tool-use JSON payload.
func parseToolUseInput(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return nil, err
	}
	if input == nil {
		return map[string]any{}, nil
	}
	return input, nil
}

// mapMessages converts shared message types into the Anthropic request shape.
func mapMessages(messages []message.Message) []anthropicMessage {
	out := make([]anthropicMessage, 0, len(messages))
	for _, msg := range messages {
		item := anthropicMessage{
			Role:    string(msg.Role),
			Content: make([]anthropicContentBlock, 0, len(msg.Content)),
		}
		for _, part := range msg.Content {
			switch part.Type {
			case "text":
				item.Content = append(item.Content, anthropicContentBlock{
					Type: "text",
					Text: part.Text,
				})
			case "tool_use":
				item.Content = append(item.Content, anthropicContentBlock{
					Type:  "tool_use",
					ID:    part.ToolUseID,
					Name:  part.ToolName,
					Input: part.ToolInput,
				})
			case "tool_result":
				item.Content = append(item.Content, anthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: part.ToolUseID,
					Content:   part.Text,
					IsError:   part.IsError,
				})
			}
		}
		if len(item.Content) == 0 {
			continue
		}
		out = append(out, item)
	}
	return out
}

// mapTools converts shared tool declarations into the Anthropic request shape.
func mapTools(tools []model.ToolDefinition) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}

	out := make([]anthropicTool, 0, len(tools))
	for _, toolDef := range tools {
		if strings.TrimSpace(toolDef.Name) == "" {
			continue
		}
		out = append(out, anthropicTool{
			Name:        toolDef.Name,
			Description: toolDef.Description,
			InputSchema: toolDef.InputSchema,
		})
	}
	return out
}
