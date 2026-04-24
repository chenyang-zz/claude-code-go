package openai

import "encoding/json"

// responsesRequest stores the minimal OpenAI Responses API request payload.
type responsesRequest struct {
	Model           string               `json:"model"`
	Input           []responsesInputItem `json:"input"`
	Tools           []responsesTool      `json:"tools,omitempty"`
	Stream          bool                 `json:"stream,omitempty"`
	MaxOutputTokens int                  `json:"max_output_tokens,omitempty"`

	// Advanced parameters
	Instructions       string                  `json:"instructions,omitempty"`
	PreviousResponseID string                  `json:"previous_response_id,omitempty"`
	Store              *bool                   `json:"store,omitempty"`
	Reasoning          *responsesReasoning     `json:"reasoning,omitempty"`
	Temperature        *float64                `json:"temperature,omitempty"`
	TopP               *float64                `json:"top_p,omitempty"`
	ToolChoice         *responsesToolChoice    `json:"tool_choice,omitempty"`
	Metadata           map[string]string       `json:"metadata,omitempty"`
	User               string                  `json:"user,omitempty"`
}

// responsesInputItem stores one conversation turn in the Responses API input array.
type responsesInputItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// responsesToolInputItem stores one function call result returned to the model.
type responsesToolInputItem struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

// responsesTool stores one function tool definition for Responses API requests.
type responsesTool struct {
	Type     string       `json:"type"`
	Function toolSpecBody `json:"function"`
}

// responsesResponse stores the non-streaming response shape from the Responses API.
type responsesResponse struct {
	ID     string                `json:"id"`
	Model  string                `json:"model"`
	Output []responsesOutputItem `json:"output"`
	Usage  *responsesUsage       `json:"usage,omitempty"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`

	// Status indicates the final state of the response.
	// Values: "completed", "in_progress", "incomplete", "failed".
	Status string `json:"status,omitempty"`
	// IncompleteDetails is present when Status is "incomplete".
	IncompleteDetails *responsesIncompleteDetails `json:"incomplete_details,omitempty"`
}

// responsesOutputItem stores one item in the response output array.
type responsesOutputItem struct {
	Type string `json:"type"` // "message" | "function_call"
	ID   string `json:"id"`

	// Fields for type="message"
	Role    string                 `json:"role,omitempty"`
	Content []responsesContentPart `json:"content,omitempty"`

	// Fields for type="function_call"
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// responsesContentPart stores one content block inside a message output item.
type responsesContentPart struct {
	Type string `json:"type"` // "output_text"
	Text string `json:"text,omitempty"`
}

// responsesUsage stores token consumption for one Responses API call.
type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// --- SSE streaming types ---

// responsesStreamEvent is the top-level envelope for every SSE line.
type responsesStreamEvent struct {
	Type        string                       `json:"type"`
	Response    *responsesResponse           `json:"response,omitempty"`
	OutputIndex int                          `json:"output_index,omitempty"`
	ContentIndex int                         `json:"content_index,omitempty"`
	Delta       string                       `json:"delta,omitempty"`
	Item        *responsesOutputItem         `json:"item,omitempty"`
	ArgumentsDelta string                    `json:"arguments_delta,omitempty"`
}

// Known SSE event type constants for the Responses API.
const (
	responsesEventCreated              = "response.created"
	responsesEventInProgress           = "response.in_progress"
	responsesEventOutputItemAdded      = "response.output_item.added"
	responsesEventOutputTextDelta      = "response.output_text.delta"
	responsesEventOutputTextDone       = "response.output_text.done"
	responsesEventOutputItemDone       = "response.output_item.done"
	responsesEventFunctionCallArgsDelta = "response.function_call_arguments.delta"
	responsesEventFunctionCallArgsDone  = "response.function_call_arguments.done"
	responsesEventCompleted            = "response.completed"
	responsesEventDone                 = "response.done"
	responsesEventIncomplete           = "response.incomplete"
	responsesEventFailed               = "response.failed"
)

// responsesReasoning controls reasoning behaviour for supported models.
type responsesReasoning struct {
	Effort string `json:"effort,omitempty"`
}

// responsesToolChoice represents the tool_choice parameter which can be
// either a string ("auto", "none", "required") or an object specifying
// a particular function.
type responsesToolChoice struct {
	// Mode is set for simple string choices.
	Mode string `json:"-"`
	// FunctionName is set when forcing a specific function.
	FunctionName string `json:"-"`
}

// MarshalJSON implements custom JSON marshalling for the union type.
func (t responsesToolChoice) MarshalJSON() ([]byte, error) {
	if t.FunctionName != "" {
		return json.Marshal(map[string]any{
			"type":     "function",
			"function": map[string]string{"name": t.FunctionName},
		})
	}
	return json.Marshal(t.Mode)
}

// responsesIncompleteDetails explains why a response ended with
// status "incomplete".
type responsesIncompleteDetails struct {
	Reason string `json:"reason"`
}
