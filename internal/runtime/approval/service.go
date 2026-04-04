package approval

import (
	"context"
	"fmt"
	"slices"
)

const (
	// ModeDefault keeps the standard interactive approval behavior.
	ModeDefault = "default"
	// ModeAcceptEdits auto-approves edit-style operations while preserving the external mode name.
	ModeAcceptEdits = "acceptEdits"
	// ModeBypassPermissions auto-approves all guarded operations.
	ModeBypassPermissions = "bypassPermissions"
	// ModeDontAsk rejects guarded operations without prompting.
	ModeDontAsk = "dontAsk"
	// ModePlan keeps the runtime in planning-first approval behavior.
	ModePlan = "plan"
)

// Request captures the minimum approval prompt context surfaced from the runtime.
type Request struct {
	// CallID identifies the tool call that triggered the approval branch.
	CallID string
	// ToolName identifies the tool requesting elevated access.
	ToolName string
	// Path carries the relevant filesystem path when one is available.
	Path string
	// Action stores a short caller-facing description such as "read" or "write".
	Action string
	// Message stores the stable approval prompt body shown to the user.
	Message string
}

// Response captures the caller decision for one approval request.
type Response struct {
	// Approved reports whether the guarded operation may continue.
	Approved bool
	// Reason stores an optional stable explanation for deny-style decisions.
	Reason string
}

// Service resolves one runtime approval request into a concrete user or policy decision.
type Service interface {
	Decide(ctx context.Context, req Request) (Response, error)
}

// Prompter renders one approval prompt and returns the caller decision.
type Prompter interface {
	Prompt(ctx context.Context, prompt Prompt) (Response, error)
}

// StaticService returns the same approval decision for every request and is useful in early-stage tests.
type StaticService struct {
	// Response stores the deterministic decision returned from Decide.
	Response Response
}

// Decide returns the preconfigured static response without additional side effects.
func (s StaticService) Decide(ctx context.Context, req Request) (Response, error) {
	_ = ctx
	_ = req
	return s.Response, nil
}

// PromptingService resolves approval requests by combining the configured approval mode with a CLI prompter when needed.
type PromptingService struct {
	// Mode stores the caller-selected external approval mode.
	Mode string
	// Prompter is invoked only for modes that still require an interactive decision.
	Prompter Prompter
}

// NewPromptingService builds a runtime approval service for the provided mode and prompt implementation.
func NewPromptingService(mode string, prompter Prompter) *PromptingService {
	return &PromptingService{
		Mode:     mode,
		Prompter: prompter,
	}
}

// Decide resolves one approval request according to the configured mode.
func (s *PromptingService) Decide(ctx context.Context, req Request) (Response, error) {
	mode := s.Mode
	if !IsSupportedMode(mode) {
		mode = ModeDefault
	}

	switch mode {
	case ModeBypassPermissions, ModeAcceptEdits:
		return Response{Approved: true}, nil
	case ModeDontAsk:
		return Response{
			Approved: false,
			Reason:   fmt.Sprintf("Permission to %s %s was not granted.", req.Action, req.Path),
		}, nil
	default:
		if s == nil || s.Prompter == nil {
			return Response{
				Approved: false,
				Reason:   "Approval service is not interactive in the current mode.",
			}, nil
		}
		return s.Prompter.Prompt(ctx, Prompt{
			Title: fmt.Sprintf("%s wants to %s", req.ToolName, req.Action),
			Body:  req.Message,
		})
	}
}

// SupportedModes returns the currently migrated approval mode names in stable order.
func SupportedModes() []string {
	return []string{
		ModeAcceptEdits,
		ModeBypassPermissions,
		ModeDefault,
		ModeDontAsk,
		ModePlan,
	}
}

// IsSupportedMode reports whether one external approval mode value is part of the migrated subset.
func IsSupportedMode(mode string) bool {
	return slices.Contains(SupportedModes(), mode)
}
