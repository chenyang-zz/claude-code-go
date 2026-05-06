// Package feedback implements the user-facing feedback submission service.
// It collects structured feedback data (description, environment info,
// sanitized transcripts) and submits it to the Anthropic feedback API.
package feedback

import "time"

// FeedbackData mirrors the TS-side FeedbackData type submitted to the API.
type FeedbackData struct {
	// LatestAssistantMessageID is the request ID of the most recent assistant message.
	LatestAssistantMessageID string `json:"latestAssistantMessageId"`
	// MessageCount is the total number of messages in the session.
	MessageCount int `json:"message_count"`
	// DateTime is the ISO 8601 timestamp of the submission.
	DateTime string `json:"datetime"`
	// Description is the user-provided feedback text.
	Description string `json:"description"`
	// Platform is the runtime platform identifier (darwin, linux, windows).
	Platform string `json:"platform"`
	// GitRepo indicates whether the current working directory is a git repository.
	GitRepo bool `json:"gitRepo"`
	// Version is the application version string.
	Version string `json:"version"`
	// Terminal is the terminal emulator identifier.
	Terminal string `json:"terminal"`
	// Transcript is the normalized message history for API submission.
	Transcript []map[string]any `json:"transcript"`
	// Errors are sanitized in-memory error logs.
	Errors []ErrorInfo `json:"errors,omitempty"`
	// SubagentTranscripts contains per-agent transcript data.
	SubagentTranscripts map[string][]map[string]any `json:"subagentTranscripts,omitempty"`
	// RawTranscriptJSONL is the raw transcript file content.
	RawTranscriptJSONL string `json:"rawTranscriptJsonl,omitempty"`
}

// ErrorInfo represents a sanitized error entry.
type ErrorInfo struct {
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// SubmissionResult is the outcome of a feedback API call.
type SubmissionResult struct {
	// Success reports whether the submission was accepted.
	Success bool
	// FeedbackID is the server-assigned identifier for the submitted feedback.
	FeedbackID string
	// IsZDROrg is true when the submission was rejected due to custom data retention policies.
	IsZDROrg bool
}

// ServiceConfig holds configuration for the Feedback service.
type ServiceConfig struct {
	// GitHubIssuesRepoURL is the base URL for creating GitHub issues.
	GitHubIssuesRepoURL string
	// GitHubURLLimit caps the total URL length for GitHub issue links.
	GitHubURLLimit int
}

// DefaultConfig returns the standard configuration.
func DefaultConfig() ServiceConfig {
	return ServiceConfig{
		GitHubIssuesRepoURL: "https://github.com/anthropics/claude-code/issues",
		GitHubURLLimit:      7250,
	}
}

// NowISO returns the current time in ISO 8601 format.
func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}
