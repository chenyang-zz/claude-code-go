package session

import (
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
)

const (
	// ContinuationPrompt stores the stable synthetic user message used when one interrupted session should continue automatically.
	ContinuationPrompt = "Continue from where you left off."
)

const (
	// InterruptionNone reports one restored history that can continue without synthetic recovery input.
	InterruptionNone InterruptionKind = "none"
	// InterruptionPrompt reports one restored history that ended on a user message before the assistant replied.
	InterruptionPrompt InterruptionKind = "interrupted_prompt"
	// InterruptionTurn reports one restored history that ended with unresolved assistant tool_use blocks.
	InterruptionTurn InterruptionKind = "interrupted_turn"
)

// InterruptionKind identifies the minimum interruption classes currently recognized during session recovery.
type InterruptionKind string

// RecoveryState carries the normalized recovery classification for one restored history.
type RecoveryState struct {
	// Kind identifies whether the restored history is complete or interrupted.
	Kind InterruptionKind
	// NeedsContinuation reports whether callers should inject one synthetic continuation prompt before resuming execution.
	NeedsContinuation bool
}

// RecoveredSnapshot carries one restored session snapshot plus the normalized interruption classification.
type RecoveredSnapshot struct {
	// Snapshot stores the cleaned normalized session history after recovery filtering.
	Snapshot coresession.Snapshot
	// State stores the minimum interruption classification produced from the restored history.
	State RecoveryState
}

// RecoveryPromptMessage builds the stable synthetic user message used to continue one interrupted session.
func RecoveryPromptMessage() message.Message {
	return message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(ContinuationPrompt),
		},
	}
}

// RecoverMessages normalizes one persisted history into an API-safe message sequence and reports whether it was interrupted.
func RecoverMessages(messages []message.Message) ([]message.Message, RecoveryState) {
	if len(messages) == 0 {
		return nil, RecoveryState{Kind: InterruptionNone}
	}

	resolvedToolResults := collectResolvedToolResults(messages)
	cleaned := make([]message.Message, 0, len(messages))
	removedUnresolvedToolUse := false

	for _, msg := range messages {
		cloned := cloneMessage(msg)
		if cloned.Role == message.RoleAssistant {
			filteredContent := make([]message.ContentPart, 0, len(cloned.Content))
			for _, part := range cloned.Content {
				if part.Type == "tool_use" && !resolvedToolResults[part.ToolUseID] {
					removedUnresolvedToolUse = true
					continue
				}
				filteredContent = append(filteredContent, cloneContentPart(part))
			}
			cloned.Content = filteredContent
		}
		if len(cloned.Content) == 0 {
			continue
		}
		cleaned = append(cleaned, cloned)
	}

	state := classifyRecoveredMessages(cleaned, removedUnresolvedToolUse)
	return cleaned, state
}

func collectResolvedToolResults(messages []message.Message) map[string]bool {
	resolved := make(map[string]bool)
	for _, msg := range messages {
		for _, part := range msg.Content {
			if part.Type != "tool_result" || part.ToolUseID == "" {
				continue
			}
			resolved[part.ToolUseID] = true
		}
	}
	return resolved
}

func classifyRecoveredMessages(messages []message.Message, removedUnresolvedToolUse bool) RecoveryState {
	if len(messages) == 0 {
		if removedUnresolvedToolUse {
			return RecoveryState{
				Kind:              InterruptionTurn,
				NeedsContinuation: true,
			}
		}
		return RecoveryState{Kind: InterruptionNone}
	}

	last := messages[len(messages)-1]
	if last.Role == message.RoleUser {
		return RecoveryState{
			Kind:              InterruptionPrompt,
			NeedsContinuation: true,
		}
	}
	if removedUnresolvedToolUse {
		return RecoveryState{
			Kind:              InterruptionTurn,
			NeedsContinuation: true,
		}
	}
	return RecoveryState{Kind: InterruptionNone}
}

func cloneMessage(msg message.Message) message.Message {
	cloned := message.Message{
		Role:    msg.Role,
		Content: make([]message.ContentPart, 0, len(msg.Content)),
	}
	for _, part := range msg.Content {
		cloned.Content = append(cloned.Content, cloneContentPart(part))
	}
	return cloned
}

func cloneContentPart(part message.ContentPart) message.ContentPart {
	cloned := part
	if part.ToolInput != nil {
		cloned.ToolInput = make(map[string]any, len(part.ToolInput))
		for key, value := range part.ToolInput {
			cloned.ToolInput[key] = value
		}
	}
	return cloned
}
