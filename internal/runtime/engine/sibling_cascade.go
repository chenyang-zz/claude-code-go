package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// bashToolName is the tool name that triggers sibling error cascade when it fails.
// Only Bash tool errors cancel sibling tools because Bash commands often have implicit
// dependency chains (e.g. mkdir fails -> subsequent commands are pointless).
// Other tools like Read/WebFetch are independent — one failure should not cancel the rest.
const bashToolName = "Bash"

// SiblingCascade manages the sibling error cascade state for concurrent tool execution.
// When a Bash tool errors during parallel execution, the cascade cancels sibling tools
// by cancelling a derived context while preserving the parent context. This mirrors the
// TS siblingAbortController mechanism in StreamingToolExecutor.ts.
//
// The cascade context hierarchy is:
//
//	parent context (engine run loop)
//	  └─> sibling context (cancelled on Bash error, does NOT propagate to parent)
type SiblingCascade struct {
	mu              sync.Mutex
	hasErrored      bool
	erroredToolDesc string
	siblingCtx      context.Context
	siblingCancel   context.CancelFunc
}

// NewSiblingCascade creates a sibling cascade with a cancel context derived from parent.
// The derived context is cancelled when TriggerBashError is called, but cancelling it
// does not affect the parent context.
func NewSiblingCascade(parentCtx context.Context) *SiblingCascade {
	ctx, cancel := context.WithCancel(parentCtx)
	return &SiblingCascade{
		siblingCtx:    ctx,
		siblingCancel: cancel,
	}
}

// TriggerBashError marks the cascade as errored and cancels the sibling context.
// All sibling tools using the cascade's Context() will receive a cancellation signal.
// toolDesc is a human-readable description of the failed tool (e.g. "Bash(mkdir /foo)").
func (sc *SiblingCascade) TriggerBashError(toolDesc string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.hasErrored = true
	sc.erroredToolDesc = toolDesc
	sc.siblingCancel()
}

// IsErrored reports whether a Bash tool has triggered the cascade.
func (sc *SiblingCascade) IsErrored() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.hasErrored
}

// ErroredToolDesc returns the description of the tool that triggered the cascade.
func (sc *SiblingCascade) ErroredToolDesc() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.erroredToolDesc
}

// Context returns the sibling context that is cancelled when a Bash tool errors.
// Sibling tools should use this context (or a further derived one) so they receive
// the cancellation signal during cascade.
func (sc *SiblingCascade) Context() context.Context {
	return sc.siblingCtx
}

// FormatCascadeErrorMessage generates a synthetic error message for a tool cancelled
// by the sibling cascade. The format matches the TS source:
//
//	"Cancelled: parallel tool call {desc} errored"
//
// When desc is empty, it returns "Cancelled: parallel tool call errored".
func FormatCascadeErrorMessage(desc string) string {
	if desc == "" {
		return "Cancelled: parallel tool call errored"
	}
	return fmt.Sprintf("Cancelled: parallel tool call %s errored", desc)
}

// FormatToolDescription generates a human-readable description of a tool call for use
// in cascade error messages. The format is "ToolName(truncated_input)" where the input
// is the concatenation of string-valued input parameters, truncated to 40 characters.
func FormatToolDescription(toolName string, input map[string]any) string {
	var parts []string
	for _, v := range input {
		if s, ok := v.(string); ok {
			parts = append(parts, s)
		}
	}
	inputStr := strings.Join(parts, " ")
	if len(inputStr) > 40 {
		inputStr = inputStr[:40] + "..."
	}
	return fmt.Sprintf("%s(%s)", toolName, inputStr)
}

// isToolErrorResult reports whether a tool execution produced an error result.
// In the TS source this checks for is_error === true on the tool_result content block.
// In Go, a tool result is an error when the invocation error is non-nil or the result
// has a non-empty Error field.
func isToolErrorResult(result coretool.Result, invokeErr error) bool {
	if invokeErr != nil {
		return true
	}
	return strings.TrimSpace(result.Error) != ""
}

// TriggerOnBashError checks if a tool error should trigger the sibling cascade.
// Only Bash tool errors trigger the cascade; other tool failures are ignored.
// This mirrors the TS design: "Only Bash errors cancel siblings. Bash commands often
// have implicit dependency chains (e.g. mkdir fails -> subsequent commands pointless).
// Read/WebFetch/etc are independent — one failure shouldn't nuke the rest."
//
// Returns true if the cascade was triggered (i.e. the tool is Bash AND produced an error).
func (sc *SiblingCascade) TriggerOnBashError(toolName string, input map[string]any, result coretool.Result, invokeErr error) bool {
	if toolName != bashToolName {
		return false
	}
	if !isToolErrorResult(result, invokeErr) {
		return false
	}
	desc := FormatToolDescription(toolName, input)
	sc.TriggerBashError(desc)
	return true
}
