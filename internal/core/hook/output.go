package hook

import (
	"encoding/json"
	"strings"
)

// Top-level hook decision constants, corresponding to TS SyncHookJSONOutput.decision.
const (
	// DecisionApprove maps to TS "approve" and implies permission allow.
	DecisionApprove = "approve"
	// DecisionBlock maps to TS "block" and implies permission deny.
	DecisionBlock = "block"
)

// Permission decision constants for PreToolUse hookSpecificOutput.permissionDecision.
const (
	// PermissionAllow allows the tool to execute without interactive approval.
	PermissionAllow = "allow"
	// PermissionDeny blocks the tool from executing.
	PermissionDeny = "deny"
	// PermissionAsk triggers the normal interactive approval flow.
	PermissionAsk = "ask"
)

// HookOutput represents the structured JSON output parsed from a hook command's stdout.
// It corresponds to SyncHookJSONOutput in the TypeScript source (coreSchemas.ts).
type HookOutput struct {
	// Continue controls conversation continuation; false requests termination.
	Continue *bool `json:"continue,omitempty"`
	// SuppressOutput suppresses hook output display when true.
	SuppressOutput *bool `json:"suppressOutput,omitempty"`
	// StopReason provides a custom stop reason for the conversation.
	StopReason *string `json:"stopReason,omitempty"`
	// Decision is the top-level permission decision: "approve" or "block".
	Decision *string `json:"decision,omitempty"`
	// SystemMessage is an optional system message injected into the conversation.
	SystemMessage *string `json:"systemMessage,omitempty"`
	// Reason is the human-readable reason for a decision.
	Reason *string `json:"reason,omitempty"`
	// HookSpecificOutput contains event-specific output fields as raw JSON.
	HookSpecificOutput json.RawMessage `json:"hookSpecificOutput,omitempty"`
}

// PreToolUseOutput contains PreToolUse-specific hook output fields parsed from
// HookSpecificOutput when hookEventName is "PreToolUse".
type PreToolUseOutput struct {
	// HookEventName is always "PreToolUse".
	HookEventName string `json:"hookEventName"`
	// PermissionDecision is "allow", "deny", or "ask".
	PermissionDecision *string `json:"permissionDecision,omitempty"`
	// PermissionDecisionReason provides context for the permission decision.
	PermissionDecisionReason *string `json:"permissionDecisionReason,omitempty"`
	// UpdatedInput contains modified tool input parameters as raw JSON.
	UpdatedInput json.RawMessage `json:"updatedInput,omitempty"`
	// AdditionalContext provides extra context string for the model.
	AdditionalContext *string `json:"additionalContext,omitempty"`
}

// ParseHookOutput parses hook stdout into a HookOutput structure.
// Returns nil if stdout is empty, not JSON, or does not start with '{'.
func ParseHookOutput(stdout string) *HookOutput {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" || stdout[0] != '{' {
		return nil
	}
	var output HookOutput
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		return nil
	}
	return &output
}

// ParsePreToolUseOutput extracts PreToolUse-specific fields from HookSpecificOutput.
// Returns nil without error if there is no HookSpecificOutput or if it does not
// contain a PreToolUse event.
func (o *HookOutput) ParsePreToolUseOutput() (*PreToolUseOutput, error) {
	if len(o.HookSpecificOutput) == 0 {
		return nil, nil
	}
	var specific PreToolUseOutput
	if err := json.Unmarshal(o.HookSpecificOutput, &specific); err != nil {
		return nil, nil
	}
	if specific.HookEventName != "PreToolUse" {
		return nil, nil
	}
	return &specific, nil
}

// ResolvePermissionBehavior determines the effective permission behavior from the
// hook output. It checks both the top-level decision field and the PreToolUse-specific
// permissionDecision field, with the specific field taking precedence.
// Returns empty string if no permission decision is made.
func (o *HookOutput) ResolvePermissionBehavior() string {
	behavior := ""

	// Top-level decision: "approve" -> "allow", "block" -> "deny".
	if o.Decision != nil {
		switch *o.Decision {
		case DecisionApprove:
			behavior = PermissionAllow
		case DecisionBlock:
			behavior = PermissionDeny
		}
	}

	// PreToolUse-specific permissionDecision overrides top-level decision.
	specific, _ := o.ParsePreToolUseOutput()
	if specific != nil && specific.PermissionDecision != nil {
		switch *specific.PermissionDecision {
		case PermissionAllow, PermissionDeny, PermissionAsk:
			behavior = *specific.PermissionDecision
		}
	}

	return behavior
}

// ResolveDenyReason returns the reason string for a deny decision.
// It checks the PreToolUse-specific reason first, then falls back to the
// top-level reason, and finally returns a default message.
func (o *HookOutput) ResolveDenyReason(command string) string {
	specific, _ := o.ParsePreToolUseOutput()
	if specific != nil && specific.PermissionDecisionReason != nil && *specific.PermissionDecisionReason != "" {
		return *specific.PermissionDecisionReason
	}
	if o.Reason != nil && *o.Reason != "" {
		return *o.Reason
	}
	return "Blocked by hook"
}

// ResolveUpdatedInput returns the updatedInput from PreToolUse-specific output.
// Returns nil if no updatedInput is provided.
func (o *HookOutput) ResolveUpdatedInput() json.RawMessage {
	specific, _ := o.ParsePreToolUseOutput()
	if specific != nil {
		return specific.UpdatedInput
	}
	return nil
}

// ResolveAdditionalContext returns the additionalContext from PreToolUse-specific output.
// Returns empty string if no additionalContext is provided.
func (o *HookOutput) ResolveAdditionalContext() string {
	specific, _ := o.ParsePreToolUseOutput()
	if specific != nil && specific.AdditionalContext != nil {
		return *specific.AdditionalContext
	}
	return ""
}

// HookPermissionResult aggregates permission decisions from multiple hook results.
// It implements the same precedence as the TS source: deny > ask > allow.
type HookPermissionResult struct {
	// Behavior is the effective permission decision: "allow", "deny", "ask", or empty.
	Behavior string
	// Reason contains the hook-provided explanation for the permission decision when available.
	Reason string
	// DenyReason contains the reason string when behavior is "deny".
	DenyReason string
	// UpdatedInput contains modified tool input when a hook provides it.
	UpdatedInput json.RawMessage
	// AdditionalContext contains extra context text from hooks to inject into the conversation.
	AdditionalContext string
}

// ResolvePreToolUsePermission aggregates permission decisions from multiple PreToolUse
// hook results. It applies precedence: deny > ask > allow, matching the TS behavior.
// Hooks that provide updatedInput without a permission decision are handled in passthrough
// mode: the updatedInput is applied to the last result that provided it.
func ResolvePreToolUsePermission(results []HookResult) HookPermissionResult {
	var aggregated HookPermissionResult
	var passthroughInput json.RawMessage
	var additionalContexts []string

	for _, r := range results {
		if r.ParsedOutput == nil {
			continue
		}
		behavior := r.ParsedOutput.ResolvePermissionBehavior()

		// Collect updatedInput from hooks that don't make a permission decision (passthrough).
		if ui := r.ParsedOutput.ResolveUpdatedInput(); len(ui) > 0 && behavior == "" {
			passthroughInput = ui
		}

		// Collect additionalContext from all hooks regardless of permission behavior.
		if ac := r.ParsedOutput.ResolveAdditionalContext(); ac != "" {
			additionalContexts = append(additionalContexts, ac)
		}

		switch behavior {
		case PermissionDeny:
			// deny always takes precedence.
			aggregated.Behavior = PermissionDeny
			aggregated.Reason = r.ParsedOutput.ResolveDenyReason("")
			aggregated.DenyReason = aggregated.Reason
		case PermissionAsk:
			// ask takes precedence over allow but not deny.
			if aggregated.Behavior != PermissionDeny {
				aggregated.Behavior = PermissionAsk
				aggregated.Reason = r.ParsedOutput.ResolveDenyReason("")
			}
		case PermissionAllow:
			// allow only if no other behavior set.
			if aggregated.Behavior == "" {
				aggregated.Behavior = PermissionAllow
			}
		}

		// Collect updatedInput from hooks that also made a permission decision.
		if ui := r.ParsedOutput.ResolveUpdatedInput(); len(ui) > 0 && behavior != "" {
			if behavior == PermissionAllow || behavior == PermissionAsk {
				aggregated.UpdatedInput = ui
			}
		}
	}

	// Apply passthrough updatedInput if no decision-bound updatedInput was set.
	if len(aggregated.UpdatedInput) == 0 && len(passthroughInput) > 0 {
		aggregated.UpdatedInput = passthroughInput
	}

	// Join all additionalContext strings, matching TS behavior.
	if len(additionalContexts) > 0 {
		aggregated.AdditionalContext = strings.Join(additionalContexts, "\n")
	}

	return aggregated
}
