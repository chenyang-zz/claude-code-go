package compact

import "encoding/json"

// Tool name constants matching TypeScript definitions.
// These names correspond to the tool identifiers used by the API.
const (
	// Shell tool names for clearable results
	ToolNameBash  = "Bash"
	ToolNameShell = "shell"

	// Search and read tools for clearable results
	ToolNameGlob      = "Glob"
	ToolNameGrep      = "Grep"
	ToolNameFileRead  = "Read"
	ToolNameWebFetch  = "WebFetch"
	ToolNameWebSearch = "WebSearch"

	// Edit tools for clearable uses
	ToolNameFileEdit     = "Edit"
	ToolNameFileWrite    = "Write"
	ToolNameNotebookEdit = "NotebookEdit"
)

// TOOLS_CLEARABLE_RESULTS lists tool names whose results (output) can be cleared.
// This includes shell tools, glob, grep, file_read, web_fetch, and web_search.
var TOOLS_CLEARABLE_RESULTS = []string{
	ToolNameBash,
	ToolNameShell,
	ToolNameGlob,
	ToolNameGrep,
	ToolNameFileRead,
	ToolNameWebFetch,
	ToolNameWebSearch,
}

// TOOLS_CLEARABLE_USES lists tool names whose entire tool-use blocks can be cleared.
// This includes file_edit, file_write, and notebook_edit.
var TOOLS_CLEARABLE_USES = []string{
	ToolNameFileEdit,
	ToolNameFileWrite,
	ToolNameNotebookEdit,
}

// TriggerConfig represents the trigger condition for a tool-clearing strategy.
type TriggerConfig struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

// KeepToolUsesConfig represents what to keep when clearing tool uses.
type KeepToolUsesConfig struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

// ClearAtLeastConfig represents the minimum amount to clear.
type ClearAtLeastConfig struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

// KeepThinkingConfig represents the keep policy for thinking blocks.
// When Type is "all", all thinking is preserved and Value is ignored.
// When Type is "thinking_turns", Value indicates how many recent turns to keep.
type KeepThinkingConfig struct {
	Type  string `json:"type"`
	Value int    `json:"value,omitempty"`
}

// clearToolUsesJSON is the JSON representation for clear_tool_uses_20250919 strategy.
type clearToolUsesJSON struct {
	Type            string             `json:"type"`
	Trigger         *TriggerConfig     `json:"trigger,omitempty"`
	Keep            *KeepToolUsesConfig `json:"keep,omitempty"`
	ClearToolInputs []string           `json:"clear_tool_inputs,omitempty"`
	ExcludeTools    []string           `json:"exclude_tools,omitempty"`
	ClearAtLeast    *ClearAtLeastConfig `json:"clear_at_least,omitempty"`
}

// clearThinkingJSON is the JSON representation for clear_thinking_20251015 strategy.
type clearThinkingJSON struct {
	Type string      `json:"type"`
	Keep interface{} `json:"keep"`
}

// ContextEditStrategy represents a single context edit operation.
// It is a union type that can represent either a tool-clearing strategy
// or a thinking-clearing strategy, distinguished by the Type field.
//
// JSON serialization handles the polymorphic "keep" field:
//   - For clear_thinking_20251015: keep can be the string "all" or an object
//   - For clear_tool_uses_20250919: keep is always an object
type ContextEditStrategy struct {
	Type string

	// Fields for clear_tool_uses_20250919
	Trigger         *TriggerConfig
	KeepToolUses    *KeepToolUsesConfig
	ClearToolInputs []string
	ExcludeTools    []string
	ClearAtLeast    *ClearAtLeastConfig

	// Field for clear_thinking_20251015
	KeepThinking *KeepThinkingConfig
}

// MarshalJSON implements custom JSON marshaling for ContextEditStrategy
// to handle the polymorphic "keep" field that differs between strategy types.
func (s ContextEditStrategy) MarshalJSON() ([]byte, error) {
	switch s.Type {
	case "clear_tool_uses_20250919":
		return json.Marshal(clearToolUsesJSON{
			Type:            s.Type,
			Trigger:         s.Trigger,
			Keep:            s.KeepToolUses,
			ClearToolInputs: s.ClearToolInputs,
			ExcludeTools:    s.ExcludeTools,
			ClearAtLeast:    s.ClearAtLeast,
		})
	case "clear_thinking_20251015":
		var keep interface{}
		if s.KeepThinking != nil {
			if s.KeepThinking.Type == "all" {
				keep = "all"
			} else {
				keep = s.KeepThinking
			}
		}
		return json.Marshal(clearThinkingJSON{
			Type: s.Type,
			Keep: keep,
		})
	default:
		// Fallback: marshal with type only
		return json.Marshal(struct {
			Type string `json:"type"`
		}{Type: s.Type})
	}
}

// ContextManagementConfig wraps one or more context edit strategies
// to be sent with an API request.
type ContextManagementConfig struct {
	Edits []ContextEditStrategy `json:"edits"`
}

// APIContextOptions holds parameters for determining which context
// management strategies to apply.
type APIContextOptions struct {
	// HasThinking indicates the conversation contains thinking blocks.
	HasThinking bool
	// IsRedactThinkingActive indicates redacted thinking is in effect,
	// meaning thinking blocks have no model-visible content.
	IsRedactThinkingActive bool
	// ClearAllThinking indicates all thinking should be cleared
	// (e.g. after a long idle period with cache miss), keeping only
	// the most recent thinking turn.
	ClearAllThinking bool
}

// GetAPIContextManagement returns the context management configuration
// for an API request based on the current conversation state.
//
// It applies thinking-clearing strategies when the conversation has
// thinking blocks and redacted thinking is not active. Returns nil
// if no strategies are applicable.
//
// Note: Tool clearing strategies are ant-only and are not included
// in this simplified implementation.
func GetAPIContextManagement(options *APIContextOptions) *ContextManagementConfig {
	if options == nil {
		options = &APIContextOptions{}
	}

	var strategies []ContextEditStrategy

	// Preserve thinking blocks in previous assistant turns. Skip when
	// redact-thinking is active -- redacted blocks have no model-visible content.
	// When ClearAllThinking is set (>1h idle = cache miss), keep only the last
	// thinking turn -- the API schema requires value >= 1, and omitting the edit
	// falls back to the model-policy default (often "all"), which wouldn't clear.
	if options.HasThinking && !options.IsRedactThinkingActive {
		var keep KeepThinkingConfig
		if options.ClearAllThinking {
			keep = KeepThinkingConfig{Type: "thinking_turns", Value: 1}
		} else {
			keep = KeepThinkingConfig{Type: "all"}
		}
		strategies = append(strategies, ContextEditStrategy{
			Type:          "clear_thinking_20251015",
			KeepThinking: &keep,
		})
	}

	if len(strategies) == 0 {
		return nil
	}
	return &ContextManagementConfig{Edits: strategies}
}
