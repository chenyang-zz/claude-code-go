package model

// EventType identifies the supported provider stream event variants.
type EventType string

const (
	// EventTypeTextDelta carries one assistant text chunk.
	EventTypeTextDelta EventType = "text_delta"
	// EventTypeThinking carries one complete assistant thinking block.
	EventTypeThinking EventType = "thinking"
	// EventTypeToolUse carries one assistant tool-use block after its JSON input is complete.
	EventTypeToolUse EventType = "tool_use"
	// EventTypeError carries a provider-facing failure converted into stream form.
	EventTypeError EventType = "error"
	// EventTypeDone marks the end of a provider stream.
	EventTypeDone EventType = "done"
	// EventTypeServerToolUse carries a server-side tool use block (e.g. web_search).
	EventTypeServerToolUse EventType = "server_tool_use"
	// EventTypeWebSearchResult carries a web_search_tool_result block from the API.
	EventTypeWebSearchResult EventType = "web_search_tool_result"
)

// StopReason describes why the model stopped generating tokens.
type StopReason string

const (
	// StopReasonEndTurn indicates the model finished its response naturally.
	StopReasonEndTurn StopReason = "end_turn"
	// StopReasonMaxTokens indicates the model hit the maximum output token limit.
	StopReasonMaxTokens StopReason = "max_tokens"
	// StopReasonToolUse indicates the model stopped to request tool execution.
	StopReasonToolUse StopReason = "tool_use"
	// StopReasonStopSequence indicates the model hit a configured stop sequence.
	StopReasonStopSequence StopReason = "stop_sequence"
)

// Event carries one provider stream item in a provider-agnostic form.
type Event struct {
	Type       EventType
	Text       string
	Thinking   string
	Signature  string
	Error      string
	ToolUse    *ToolUse
	// ServerToolUse carries a server-side tool use block emitted by the model
	// when it invokes a server-defined tool such as web_search_20250305.
	ServerToolUse *ServerToolUse
	// WebSearchResult carries a web_search_tool_result block from the API
	// containing search result hits or an error.
	WebSearchResult *WebSearchResult
	StopReason      StopReason
	Usage           *Usage
	// ResponseID carries the provider-side response identifier when available.
	// Used by the OpenAI Responses API for stateful conversation tracking.
	ResponseID string
}

// ServerToolUse represents a server-side tool invocation requested by the model,
// such as a web_search_20250305 call.
type ServerToolUse struct {
	ID    string
	Name  string
	Input map[string]any
}

// WebSearchResult represents the result of a web_search_20250305 execution.
type WebSearchResult struct {
	ToolUseID string
	Content   []WebSearchHit
	ErrorCode string
}

// WebSearchHit carries a single web search result entry.
type WebSearchHit struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Stream is the asynchronous event channel returned by a model client.
type Stream <-chan Event
