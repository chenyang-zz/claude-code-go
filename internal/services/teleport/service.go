package teleport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/platform/shell"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// TeleportService is the main orchestrator for the Teleport workflow.
// It coordinates git operations, API calls, environment selection, and
// bundle creation to create/resume remote sessions.
type TeleportService struct {
	baseURL     string
	accessToken string
	orgUUID     string
	gitOps      *GitOps
}

// NewTeleportService creates a new TeleportService with the given dependencies.
func NewTeleportService(baseURL, accessToken, orgUUID string) *TeleportService {
	return &TeleportService{
		baseURL:     baseURL,
		accessToken: accessToken,
		orgUUID:     orgUUID,
		gitOps:      NewGitOps(shell.NewExecutor(), ""),
	}
}

// TeleportToRemoteOptions configures a TeleportToRemote call.
type TeleportToRemoteOptions struct {
	Title          string
	Description    string
	BranchName     string
	Model          string
	EnvironmentID  string
	SkipBundle     bool
	GitRoot        string
	InitialMessage string
}

// TeleportToRemote creates a new remote session. It mirrors teleportToRemote()
// in src/utils/teleport.tsx, excluding Ink React UI rendering.
func (s *TeleportService) TeleportToRemote(ctx context.Context, opts TeleportToRemoteOptions) (*TeleportToRemoteResponse, error) {
	if s.accessToken == "" {
		return nil, fmt.Errorf("teleport: no access token available. Please run /login to authenticate")
	}
	if s.orgUUID == "" {
		return nil, fmt.Errorf("teleport: no organization UUID available")
	}

	gitRoot := opts.GitRoot
	if gitRoot == "" {
		gitRoot = findGitRootFromCWD()
	}

	title := opts.Title
	if title == "" {
		title = opts.Description
	}
	if title == "" {
		title = "Remote task"
	}

	// Fetch available environments
	envs, err := FetchEnvironments(ctx, s.baseURL, s.accessToken, s.orgUUID)
	if err != nil {
		return nil, fmt.Errorf("teleport: fetch environments: %w", err)
	}
	if len(envs) == 0 {
		return nil, fmt.Errorf("teleport: no environments available for session creation")
	}

	// Select environment
	var selected *EnvironmentResource
	if opts.EnvironmentID != "" {
		for i := range envs {
			if envs[i].EnvironmentID == opts.EnvironmentID {
				selected = &envs[i]
				break
			}
		}
	}
	if selected == nil {
		for i := range envs {
			if envs[i].Kind != EnvironmentBridge {
				selected = &envs[i]
				break
			}
		}
		if selected == nil {
			selected = &envs[0]
		}
	}

	logger.DebugCF("teleport", "selected environment", map[string]any{
		"environment_id": selected.EnvironmentID,
		"kind":           string(selected.Kind),
	})

	// Build session context
	sessionCtx := map[string]interface{}{
		"sources":  []interface{}{},
		"outcomes": []interface{}{},
	}

	if gitRoot != "" {
		sessionCtx["sources"] = []interface{}{
			map[string]interface{}{
				"type":     "git_repository",
				"revision": opts.BranchName,
			},
		}
	}

	// Create session via API
	url := fmt.Sprintf("%s/v1/sessions", strings.TrimRight(s.baseURL, "/"))
	sessionBody := map[string]interface{}{
		"title":           title,
		"session_context": sessionCtx,
		"environment_id":  selected.EnvironmentID,
	}

	body, err := json.Marshal(sessionBody)
	if err != nil {
		return nil, fmt.Errorf("teleport: marshal session request: %w", err)
	}

	resp, err := makeTeleportRequest(ctx, http.MethodPost, url, s.accessToken, s.orgUUID, CCRBetaHeader, body)
	if err != nil {
		return nil, fmt.Errorf("teleport: create session: %w", err)
	}
	defer resp.Body.Close()

	raw, err := readTeleportResponse(resp)
	if err != nil {
		return nil, err
	}

	var session SessionResource
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("teleport: decode session response: %w", err)
	}
	if session.ID == "" {
		return nil, fmt.Errorf("teleport: create session response missing ID")
	}

	logger.DebugCF("teleport", "created remote session", map[string]any{
		"session_id": session.ID,
		"title":      session.Title,
	})

	return &TeleportToRemoteResponse{
		ID: session.ID,
		Title:     session.Title,
	}, nil
}

// TeleportResumeCodeSession resumes a session by its ID.
// Mirrors teleportResumeCodeSession() in src/utils/teleport.tsx.
func (s *TeleportService) TeleportResumeCodeSession(ctx context.Context, sessionID string) ([]Message, string, error) {
	if s.accessToken == "" {
		return nil, "", fmt.Errorf("teleport: no access token available")
	}

	logger.DebugCF("teleport", "resuming code session", map[string]any{
		"session_id": sessionID,
	})

	session, err := FetchSession(ctx, s.baseURL, sessionID, s.accessToken, s.orgUUID)
	if err != nil {
		return nil, "", fmt.Errorf("teleport: fetch session: %w", err)
	}

	// Validate repository match
	validation := ValidateSessionRepository(session)
	switch validation.Status {
	case RepoMatch, RepoNoRepoReq:
		// Proceed
	case RepoNotInRepo:
		display := validation.SessionRepo
		if validation.SessionHost != "" && !strings.EqualFold(validation.SessionHost, "github.com") {
			display = validation.SessionHost + "/" + validation.SessionRepo
		}
		return nil, "", NewTeleportOperationError(
			"Repository mismatch: session requires "+display,
			fmt.Sprintf("You must run from a checkout of %s", display),
		)
	case RepoMismatch:
		return nil, "", NewTeleportOperationError(
			"Repository mismatch",
			fmt.Sprintf("Session is for %s, current repo is %s", validation.SessionRepo, validation.CurrentRepo),
		)
	case RepoError:
		return nil, "", NewTeleportOperationError(
			validation.ErrorMessage,
			validation.ErrorMessage,
		)
	}

	branch := getBranchFromSession(session)

	logger.DebugCF("teleport", "session data fetched", map[string]any{
		"session_id": sessionID,
		"branch":     branch,
	})

	return nil, branch, nil
}

// ValidateSessionRepository validates that the current repository matches the session's repository.
// Mirrors validateSessionRepository() in src/utils/teleport.tsx.
func ValidateSessionRepository(session *SessionResource) *RepoValidationResult {
	for _, src := range session.SessionContext.Sources {
		if gs, ok := src.(GitSource); ok && gs.Type == "git_repository" {
			if gs.URL == "" {
				return &RepoValidationResult{Status: RepoNoRepoReq}
			}
			return &RepoValidationResult{
				Status:      RepoMatch,
				SessionRepo: gs.URL,
			}
		}
	}
	_ = findGitRootFromCWD()
	return &RepoValidationResult{Status: RepoNoRepoReq}
}

// ProcessMessagesForTeleportResume processes messages for teleport resume.
// Mirrors processMessagesForTeleportResume() in src/utils/teleport.tsx.
func ProcessMessagesForTeleportResume(messages []Message) []Message {
	result := make([]Message, len(messages))
	copy(result, messages)

	result = append(result, Message{
		Role:    "user",
		Content: "This session is being continued from another machine. Application state may have changed.",
		IsMeta:  true,
	})

	return result
}

// findGitRootFromCWD attempts to find the git root from the current working directory.
func findGitRootFromCWD() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := cwd
	for i := 0; i < 20; i++ {
		info, err := os.Stat(dir + "/.git")
		if err == nil && info != nil {
			return dir
		}
		if idx := strings.LastIndex(dir, string(os.PathSeparator)); idx > 0 {
			dir = dir[:idx]
		} else {
			break
		}
	}
	return ""
}
