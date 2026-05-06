// Package teleport implements the Teleport service for remote session management.
//
// Teleport allows transferring a Claude Code session to a remote compute
// environment (CCR), enabling continued work in the cloud. It handles
// session creation, resume from a remote session ID, event polling, and
// session archiving.
package teleport

import "encoding/json"

// TeleportResult is the outcome of a teleported session resume.
type TeleportResult struct {
	Messages   []Message
	BranchName string
}

// Message represents a conversation message exchanged during a session.
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
	IsMeta  bool        `json:"is_meta,omitempty"`
}

// TeleportProgressStep enumerates the steps during teleport progress reporting.
type TeleportProgressStep string

const (
	ProgressValidating   TeleportProgressStep = "validating"
	ProgressFetchingLogs TeleportProgressStep = "fetching_logs"
	ProgressFetchingBranch TeleportProgressStep = "fetching_branch"
	ProgressCheckingOut  TeleportProgressStep = "checking_out"
	ProgressDone         TeleportProgressStep = "done"
)

// TeleportProgressCallback is the callback signature for progress updates.
type TeleportProgressCallback func(step TeleportProgressStep)

// TitleAndBranch holds a generated session title and git branch name.
type TitleAndBranch struct {
	Title      string
	BranchName string
}

// TeleportToRemoteResponse is returned after successfully creating a remote session.
type TeleportToRemoteResponse struct {
	ID    string
	Title string
}

// RepoValidationResult holds the result of validating a session's repository.
type RepoValidationResult struct {
	Status         RepoValidationStatus
	SessionRepo    string `json:"session_repo,omitempty"`
	CurrentRepo    string `json:"current_repo,omitempty"`
	SessionHost    string `json:"session_host,omitempty"`
	CurrentHost    string `json:"current_host,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

// RepoValidationStatus describes the outcome of repository validation.
type RepoValidationStatus string

const (
	RepoMatch        RepoValidationStatus = "match"
	RepoMismatch     RepoValidationStatus = "mismatch"
	RepoNotInRepo    RepoValidationStatus = "not_in_repo"
	RepoNoRepoReq    RepoValidationStatus = "no_repo_required"
	RepoError        RepoValidationStatus = "error"
)

// PollRemoteSessionResponse holds the result of polling a remote session for new events.
type PollRemoteSessionResponse struct {
	NewEvents    []SDKMessage `json:"new_events"`
	LastEventID  string       `json:"last_event_id,omitempty"`
	Branch       string       `json:"branch,omitempty"`
	SessionStatus SessionStatus `json:"session_status,omitempty"`
}

// SDKMessage represents a message in the SDK event format.
type SDKMessage struct {
	SessionID string      `json:"session_id,omitempty"`
	Type      string      `json:"type,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
}

// SessionStatus describes the current state of a remote session.
type SessionStatus string

const (
	SessionRequiresAction SessionStatus = "requires_action"
	SessionRunning        SessionStatus = "running"
	SessionIdle           SessionStatus = "idle"
	SessionArchived       SessionStatus = "archived"
)

// GitSource describes a git repository source for a session.
type GitSource struct {
	Type                      string `json:"type"`
	URL                       string `json:"url"`
	Revision                  string `json:"revision,omitempty"`
	AllowUnrestrictedGitPush  bool   `json:"allow_unrestricted_git_push,omitempty"`
}

// KnowledgeBaseSource describes a knowledge base source for a session.
type KnowledgeBaseSource struct {
	Type             string `json:"type"`
	KnowledgeBaseID  string `json:"knowledge_base_id"`
}

// SessionContextSource is either a GitSource or KnowledgeBaseSource.
type SessionContextSource interface {
	isSessionContextSource()
}

func (GitSource) isSessionContextSource()          {}
func (KnowledgeBaseSource) isSessionContextSource()  {}

// OutcomeGitInfo holds git outcome information for a session.
type OutcomeGitInfo struct {
	Type     string   `json:"type"`
	Repo     string   `json:"repo"`
	Branches []string `json:"branches"`
}

// GitRepositoryOutcome describes the outcome of a git repository session.
type GitRepositoryOutcome struct {
	Type    string         `json:"type"`
	GitInfo OutcomeGitInfo `json:"git_info"`
}

// SessionContext holds the full context for creating or describing a session.
type SessionContext struct {
	Sources           []SessionContextSource `json:"sources"`
	CWD               string                 `json:"cwd"`
	Outcomes          []GitRepositoryOutcome `json:"outcomes,omitempty"`
	CustomSystemPrompt string                `json:"custom_system_prompt,omitempty"`
	AppendSystemPrompt string                `json:"append_system_prompt,omitempty"`
	Model              string                `json:"model,omitempty"`
	SeedBundleFileID   string                `json:"seed_bundle_file_id,omitempty"`
	GitHubPR           *GitHubPRInfo         `json:"github_pr,omitempty"`
	ReuseOutcomeBranches bool               `json:"reuse_outcome_branches,omitempty"`
	EnvironmentVariables map[string]string   `json:"environment_variables,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for SessionContext.
// SessionContextSource is an interface (encoding/json cannot unmarshal into
// interfaces with methods), so the Sources field must be decoded by inspecting
// the "type" discriminator on each element.
func (sc *SessionContext) UnmarshalJSON(data []byte) error {
	type sessionContextAlias SessionContext
	aux := &struct {
		Sources []json.RawMessage `json:"sources"`
		*sessionContextAlias
	}{
		sessionContextAlias: (*sessionContextAlias)(sc),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	for _, raw := range aux.Sources {
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var typeHolder struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &typeHolder); err != nil {
			return err
		}
		var source SessionContextSource
		switch typeHolder.Type {
		case "git":
			var g GitSource
			if err := json.Unmarshal(raw, &g); err != nil {
				return err
			}
			source = g
		case "knowledge_base":
			var kb KnowledgeBaseSource
			if err := json.Unmarshal(raw, &kb); err != nil {
				return err
			}
			source = kb
		}
		if source != nil {
			sc.Sources = append(sc.Sources, source)
		}
	}
	return nil
}

// GitHubPRInfo holds GitHub PR information attached to a session.
type GitHubPRInfo struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

// SessionResource is the API representation of a remote session.
type SessionResource struct {
	Type           string         `json:"type"`
	ID             string         `json:"id"`
	Title          string         `json:"title,omitempty"`
	SessionStatus  SessionStatus  `json:"session_status"`
	EnvironmentID  string         `json:"environment_id"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
	SessionContext SessionContext `json:"session_context"`
}

// ListSessionsResponse is the API response for listing sessions.
type ListSessionsResponse struct {
	Data    []SessionResource `json:"data"`
	HasMore bool              `json:"has_more"`
	FirstID string            `json:"first_id,omitempty"`
	LastID  string            `json:"last_id,omitempty"`
}

// EnvironmentKind describes the type of compute environment.
type EnvironmentKind string

const (
	EnvironmentAnthropicCloud EnvironmentKind = "anthropic_cloud"
	EnvironmentBYOC           EnvironmentKind = "byoc"
	EnvironmentBridge         EnvironmentKind = "bridge"
)

// EnvironmentResource describes a compute environment available for sessions.
type EnvironmentResource struct {
	Kind          EnvironmentKind `json:"kind"`
	EnvironmentID string          `json:"environment_id"`
	Name          string          `json:"name"`
	CreatedAt     string          `json:"created_at"`
	State         string          `json:"state"`
}

// EnvironmentListResponse is the API response for listing environments.
type EnvironmentListResponse struct {
	Environments []EnvironmentResource `json:"environments"`
	HasMore      bool                   `json:"has_more"`
	FirstID      string                 `json:"first_id,omitempty"`
	LastID       string                 `json:"last_id,omitempty"`
}

// EnvironmentSelectionInfo holds environment selection details.
type EnvironmentSelectionInfo struct {
	AvailableEnvironments     []EnvironmentResource `json:"available_environments"`
	SelectedEnvironment       *EnvironmentResource  `json:"selected_environment,omitempty"`
	SelectedEnvironmentSource string                `json:"selected_environment_source,omitempty"`
}

// BundleUploadResult is the outcome of a git bundle creation and upload attempt.
type BundleUploadResult struct {
	Success         bool            `json:"success"`
	FileID          string          `json:"file_id,omitempty"`
	BundleSizeBytes int64           `json:"bundle_size_bytes,omitempty"`
	Scope           BundleScope     `json:"scope,omitempty"`
	HasWip          bool            `json:"has_wip,omitempty"`
	Error           string          `json:"error,omitempty"`
	FailReason      BundleFailReason `json:"fail_reason,omitempty"`
}

// BundleScope describes the scope of a git bundle.
type BundleScope string

const (
	BundleScopeAll      BundleScope = "all"
	BundleScopeHead     BundleScope = "head"
	BundleScopeSquashed BundleScope = "squashed"
)

// BundleFailReason describes why a bundle operation failed.
type BundleFailReason string

const (
	BundleFailGitError   BundleFailReason = "git_error"
	BundleFailTooLarge   BundleFailReason = "too_large"
	BundleFailEmptyRepo  BundleFailReason = "empty_repo"
)

// TeleportOperationError is a teleport-specific error with user-facing messages.
type TeleportOperationError struct {
	Message       string
	UserMessage   string
}

func (e *TeleportOperationError) Error() string {
	return e.Message
}

// UserFacingMessage returns the error message suitable for display to the user.
func (e *TeleportOperationError) UserFacingMessage() string {
	if e.UserMessage != "" {
		return e.UserMessage
	}
	return e.Message
}

// NewTeleportOperationError creates a new TeleportOperationError.
func NewTeleportOperationError(msg, userMsg string) *TeleportOperationError {
	return &TeleportOperationError{Message: msg, UserMessage: userMsg}
}

// RemoteMessageContent is a string or content blocks for sending to a remote session.
type RemoteMessageContent struct {
	Text    string          `json:"text,omitempty"`
	Blocks  []ContentBlock  `json:"blocks,omitempty"`
}

// ContentBlock is a content block in a remote session message.
type ContentBlock struct {
	Type string         `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// CCRBetaHeader is the anthropic-beta header value for CCR BYOC.
const CCRBetaHeader = "ccr-byoc-2025-07-29"
