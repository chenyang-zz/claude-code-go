package hook

import (
	"encoding/json"
	"testing"
)

func TestParseHookOutput(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		want    *HookOutput
		wantNil bool
	}{
		{
			name:    "empty stdout",
			stdout:  "",
			wantNil: true,
		},
		{
			name:    "plain text",
			stdout:  "some text",
			wantNil: true,
		},
		{
			name:    "invalid JSON",
			stdout:  "{broken",
			wantNil: true,
		},
		{
			name:   "empty object",
			stdout: "{}",
			want:   &HookOutput{},
		},
		{
			name:   "continue false",
			stdout: `{"continue": false}`,
			want: &HookOutput{
				Continue: func() *bool { b := false; return &b }(),
			},
		},
		{
			name:   "decision approve",
			stdout: `{"decision": "approve"}`,
			want: &HookOutput{
				Decision: func() *string { s := "approve"; return &s }(),
			},
		},
		{
			name:   "decision block with reason",
			stdout: `{"decision": "block", "reason": "not allowed"}`,
			want: &HookOutput{
				Decision: func() *string { s := "block"; return &s }(),
				Reason:   func() *string { s := "not allowed"; return &s }(),
			},
		},
		{
			name:   "full PreToolUse output",
			stdout: `{"decision":"approve","hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","updatedInput":{"file_path":"/new/path"},"permissionDecisionReason":"rerouted"}}`,
			want: &HookOutput{
				Decision: func() *string { s := "approve"; return &s }(),
				HookSpecificOutput: json.RawMessage(
					`{"hookEventName":"PreToolUse","permissionDecision":"allow","updatedInput":{"file_path":"/new/path"},"permissionDecisionReason":"rerouted"}`,
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseHookOutput(tt.stdout)
			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseHookOutput() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseHookOutput() = nil, want non-nil")
			}
			// Check Continue field.
			if (got.Continue == nil) != (tt.want.Continue == nil) {
				t.Errorf("Continue: got %v, want %v", got.Continue, tt.want.Continue)
			} else if got.Continue != nil && *got.Continue != *tt.want.Continue {
				t.Errorf("Continue: got %v, want %v", *got.Continue, *tt.want.Continue)
			}
			// Check Decision field.
			if (got.Decision == nil) != (tt.want.Decision == nil) {
				t.Errorf("Decision: got %v, want %v", got.Decision, tt.want.Decision)
			} else if got.Decision != nil && *got.Decision != *tt.want.Decision {
				t.Errorf("Decision: got %v, want %v", *got.Decision, *tt.want.Decision)
			}
		})
	}
}

func TestParsePreToolUseOutput(t *testing.T) {
	tests := []struct {
		name        string
		hookOutput  *HookOutput
		wantNil     bool
		perm        string
		updatedFile string
	}{
		{
			name:       "no hookSpecificOutput",
			hookOutput: &HookOutput{},
			wantNil:    true,
		},
		{
			name: "wrong event name",
			hookOutput: &HookOutput{
				HookSpecificOutput: json.RawMessage(`{"hookEventName":"PostToolUse"}`),
			},
			wantNil: true,
		},
		{
			name: "allow decision with updatedInput",
			hookOutput: &HookOutput{
				HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"allow","updatedInput":{"file_path":"/new/path"}}`),
			},
			wantNil:     false,
			perm:        "allow",
			updatedFile: "/new/path",
		},
		{
			name: "deny decision with reason",
			hookOutput: &HookOutput{
				HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"forbidden"}`),
			},
			wantNil: false,
			perm:    "deny",
		},
		{
			name: "ask decision",
			hookOutput: &HookOutput{
				HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"ask"}`),
			},
			wantNil: false,
			perm:    "ask",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.hookOutput.ParsePreToolUseOutput()
			if err != nil {
				t.Fatalf("ParsePreToolUseOutput() error = %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("ParsePreToolUseOutput() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParsePreToolUseOutput() = nil, want non-nil")
			}
			if tt.perm != "" {
				if got.PermissionDecision == nil || *got.PermissionDecision != tt.perm {
					t.Errorf("PermissionDecision = %v, want %v", got.PermissionDecision, tt.perm)
				}
			}
			if tt.updatedFile != "" {
				var input map[string]string
				if err := json.Unmarshal(got.UpdatedInput, &input); err != nil {
					t.Fatalf("unmarshal updatedInput: %v", err)
				}
				if input["file_path"] != tt.updatedFile {
					t.Errorf("updatedInput file_path = %v, want %v", input["file_path"], tt.updatedFile)
				}
			}
		})
	}
}

func TestResolvePermissionBehavior(t *testing.T) {
	tests := []struct {
		name       string
		hookOutput *HookOutput
		want       string
	}{
		{
			name:       "no decision",
			hookOutput: &HookOutput{},
			want:       "",
		},
		{
			name:       "top-level approve",
			hookOutput: &HookOutput{Decision: func() *string { s := "approve"; return &s }()},
			want:       PermissionAllow,
		},
		{
			name:       "top-level block",
			hookOutput: &HookOutput{Decision: func() *string { s := "block"; return &s }()},
			want:       PermissionDeny,
		},
		{
			name: "specific overrides top-level",
			hookOutput: &HookOutput{
				Decision:          func() *string { s := "approve"; return &s }(),
				HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"deny"}`),
			},
			want: PermissionDeny,
		},
		{
			name: "specific allow",
			hookOutput: &HookOutput{
				HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"allow"}`),
			},
			want: PermissionAllow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hookOutput.ResolvePermissionBehavior()
			if got != tt.want {
				t.Errorf("ResolvePermissionBehavior() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveDenyReason(t *testing.T) {
	tests := []struct {
		name       string
		hookOutput *HookOutput
		command    string
		want       string
	}{
		{
			name:       "default reason",
			hookOutput: &HookOutput{},
			command:    "test-cmd",
			want:       "Blocked by hook",
		},
		{
			name: "top-level reason",
			hookOutput: &HookOutput{
				Reason: func() *string { s := "top reason"; return &s }(),
			},
			command: "test-cmd",
			want:    "top reason",
		},
		{
			name: "specific reason overrides top-level",
			hookOutput: &HookOutput{
				Reason:             func() *string { s := "top reason"; return &s }(),
				HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecisionReason":"specific reason"}`),
			},
			command: "test-cmd",
			want:    "specific reason",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hookOutput.ResolveDenyReason(tt.command)
			if got != tt.want {
				t.Errorf("ResolveDenyReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePreToolUsePermission(t *testing.T) {
	deny := "deny"

	tests := []struct {
		name               string
		results            []HookResult
		wantBehav          string
		wantReason         string
		wantUpdated        bool
		wantAdditionalCtx  string
	}{
		{
			name:        "no results",
			results:     nil,
			wantBehav:   "",
			wantReason:  "",
			wantUpdated: false,
		},
		{
			name: "no parsed output",
			results: []HookResult{
				{ExitCode: 0, Stdout: "plain text"},
			},
			wantBehav:   "",
			wantReason:  "",
			wantUpdated: false,
		},
		{
			name: "single deny from stdout",
			results: []HookResult{
				{
					ExitCode: 0,
					Stdout:   `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"forbidden"}}`,
					ParsedOutput: &HookOutput{
						HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"forbidden"}`),
					},
				},
			},
			wantBehav:   PermissionDeny,
			wantReason:  "forbidden",
			wantUpdated: false,
		},
		{
			name: "allow with updatedInput",
			results: []HookResult{
				{
					ExitCode: 0,
					Stdout:   `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","updatedInput":{"path":"/new"}}}`,
					ParsedOutput: &HookOutput{
						HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"allow","updatedInput":{"path":"/new"}}`),
					},
				},
			},
			wantBehav:   PermissionAllow,
			wantReason:  "",
			wantUpdated: true,
		},
		{
			name: "deny overrides allow",
			results: []HookResult{
				{
					ParsedOutput: &HookOutput{
						HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"allow"}`),
					},
				},
				{
					ParsedOutput: &HookOutput{
						Reason:             &deny,
						HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"blocked"}`),
					},
				},
			},
			wantBehav:  PermissionDeny,
			wantReason: "blocked",
		},
		{
			name: "top-level block decision",
			results: []HookResult{
				{
					ParsedOutput: &HookOutput{
						Decision: func() *string { s := "block"; return &s }(),
						Reason:   func() *string { s := "not allowed"; return &s }(),
					},
				},
			},
			wantBehav:  PermissionDeny,
			wantReason: "not allowed",
		},
		{
			name: "passthrough updatedInput without decision",
			results: []HookResult{
				{
					ParsedOutput: &HookOutput{
						HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","updatedInput":{"file":"x.go"}}`),
					},
				},
			},
			wantBehav:   "",
			wantReason:  "",
			wantUpdated: true,
		},
		{
			name: "additionalContext collected from single hook",
			results: []HookResult{
				{
					ParsedOutput: &HookOutput{
						HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"allow","additionalContext":"extra info"}`),
					},
				},
			},
			wantBehav:         PermissionAllow,
			wantAdditionalCtx: "extra info",
		},
		{
			name: "additionalContext joined from multiple hooks",
			results: []HookResult{
				{
					ParsedOutput: &HookOutput{
						HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"allow","additionalContext":"ctx1"}`),
					},
				},
				{
					ParsedOutput: &HookOutput{
						HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","additionalContext":"ctx2"}`),
					},
				},
			},
			wantBehav:         PermissionAllow,
			wantAdditionalCtx: "ctx1\nctx2",
		},
		{
			name: "additionalContext collected even with deny",
			results: []HookResult{
				{
					ParsedOutput: &HookOutput{
						HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"no","additionalContext":"denied info"}`),
					},
				},
			},
			wantBehav:         PermissionDeny,
			wantReason:        "no",
			wantAdditionalCtx: "denied info",
		},
		{
			name: "ask with additionalContext",
			results: []HookResult{
				{
					ParsedOutput: &HookOutput{
						HookSpecificOutput: json.RawMessage(`{"hookEventName":"PreToolUse","permissionDecision":"ask","additionalContext":"please review"}`),
					},
				},
			},
			wantBehav:         PermissionAsk,
			wantAdditionalCtx: "please review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Populate ParsedOutput from Stdout when not explicitly set.
			for i := range tt.results {
				if tt.results[i].ParsedOutput == nil && tt.results[i].Stdout != "" {
					tt.results[i].ParsedOutput = ParseHookOutput(tt.results[i].Stdout)
				}
			}

			got := ResolvePreToolUsePermission(tt.results)
			if got.Behavior != tt.wantBehav {
				t.Errorf("Behavior = %q, want %q", got.Behavior, tt.wantBehav)
			}
			if got.DenyReason != tt.wantReason {
				t.Errorf("DenyReason = %q, want %q", got.DenyReason, tt.wantReason)
			}
			if tt.wantUpdated && len(got.UpdatedInput) == 0 {
				t.Error("expected UpdatedInput, got empty")
			}
			if !tt.wantUpdated && len(got.UpdatedInput) > 0 {
				t.Errorf("expected no UpdatedInput, got %s", string(got.UpdatedInput))
			}
			if got.AdditionalContext != tt.wantAdditionalCtx {
				t.Errorf("AdditionalContext = %q, want %q", got.AdditionalContext, tt.wantAdditionalCtx)
			}
		})
	}
}

func TestPostToolFailureHookInputFieldNames(t *testing.T) {
	// Verify the input serializes with TS-compatible field names.
	input := PostToolFailureHookInput{
		BaseHookInput: BaseHookInput{
			SessionID:      "sess-1",
			TranscriptPath: "/tmp/transcript",
			CWD:            "/home",
		},
		HookEventName: "PostToolUseFailure",
		ToolName:      "Read",
		ToolInput:     json.RawMessage(`{"file_path":"test.go"}`),
		Error:         "permission denied",
		IsInterrupt:   true,
		ToolUseID:     "toolu_1",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	// Verify field names match TS expectations.
	if _, ok := raw["error"]; !ok {
		t.Error("missing 'error' field in serialized output")
	}
	if _, ok := raw["tool_error"]; ok {
		t.Error("should not have 'tool_error' field; expected 'error'")
	}
	if _, ok := raw["is_interrupt"]; !ok {
		t.Error("missing 'is_interrupt' field in serialized output")
	}
	if raw["hook_event_name"] != "PostToolUseFailure" {
		t.Errorf("hook_event_name = %v, want PostToolUseFailure", raw["hook_event_name"])
	}
}
