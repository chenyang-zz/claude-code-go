package teleport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	teleportAPITimeout = 30 * time.Second
	anthropicVersion   = "2023-06-01"
)

// makeTeleportRequest is a helper for making authenticated HTTP requests.
func makeTeleportRequest(ctx context.Context, method, url, accessToken, orgUUID, beta string, body []byte) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("teleport: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", anthropicVersion)
	if beta != "" {
		req.Header.Set("anthropic-beta", beta)
	}
	if orgUUID != "" {
		req.Header.Set("x-organization-uuid", orgUUID)
	}
	client := &http.Client{Timeout: teleportAPITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("teleport: request failed: %w", err)
	}
	return resp, nil
}

// readTeleportResponse reads the response body and checks for errors.
func readTeleportResponse(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("teleport: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("teleport: request failed (status %d %s): %s",
			resp.StatusCode, http.StatusText(resp.StatusCode), strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

// FetchSession fetches a single session by ID from the Sessions API.
// Mirrors fetchSession() in src/utils/teleport/api.ts.
func FetchSession(ctx context.Context, baseURL, sessionID, accessToken, orgUUID string) (*SessionResource, error) {
	url := fmt.Sprintf("%s/v1/sessions/%s", strings.TrimRight(baseURL, "/"), sessionID)
	resp, err := makeTeleportRequest(ctx, http.MethodGet, url, accessToken, orgUUID, CCRBetaHeader, nil)
	if err != nil {
		return nil, err
	}
	raw, err := readTeleportResponse(resp)
	if err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("teleport: session not found: %s", sessionID)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("teleport: session expired. Please run /login to sign in again")
		}
		return nil, err
	}
	var session SessionResource
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("teleport: decode session response: %w", err)
	}
	return &session, nil
}

// getBranchFromSession extracts the first branch name from a session's git repository outcomes.
// Mirrors getBranchFromSession() in src/utils/teleport/api.ts.
func getBranchFromSession(session *SessionResource) string {
	outcomes := session.SessionContext.Outcomes
	for i := range outcomes {
		if outcomes[i].Type == "git_repository" {
			branches := outcomes[i].GitInfo.Branches
			if len(branches) > 0 {
				return branches[0]
			}
		}
	}
	return ""
}

// SendEventToRemoteSession sends a user message event to an existing remote session.
// Mirrors sendEventToRemoteSession() in src/utils/teleport/api.ts.
func SendEventToRemoteSession(ctx context.Context, baseURL, sessionID, accessToken, orgUUID string, messageContent string) bool {
	url := fmt.Sprintf("%s/v1/sessions/%s/events", strings.TrimRight(baseURL, "/"), sessionID)

	eventPayload := map[string]interface{}{
		"session_id": sessionID,
		"type":       "user",
		"message": map[string]interface{}{
			"role":    "user",
			"content": messageContent,
		},
	}
	requestBody := map[string]interface{}{
		"events": []interface{}{eventPayload},
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		logger.DebugCF("teleport", "send event marshal error", map[string]any{"error": err.Error()})
		return false
	}

	resp, err := makeTeleportRequest(ctx, http.MethodPost, url, accessToken, orgUUID, CCRBetaHeader, payload)
	if err != nil {
		logger.DebugCF("teleport", "send event request error", map[string]any{"error": err.Error()})
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		logger.DebugCF("teleport", "successfully sent event to session", map[string]any{
			"session_id": sessionID,
		})
		return true
	}

	rawBody, _ := io.ReadAll(resp.Body)
	logger.DebugCF("teleport", "send event failed", map[string]any{
		"status":     resp.StatusCode,
		"response":   string(rawBody),
	})
	return false
}

// UpdateSessionTitle updates the title of an existing remote session.
// Mirrors updateSessionTitle() in src/utils/teleport/api.ts.
func UpdateSessionTitle(ctx context.Context, baseURL, sessionID, accessToken, orgUUID, title string) bool {
	url := fmt.Sprintf("%s/v1/sessions/%s", strings.TrimRight(baseURL, "/"), sessionID)

	payload, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		logger.DebugCF("teleport", "update title marshal error", map[string]any{"error": err.Error()})
		return false
	}

	resp, err := makeTeleportRequest(ctx, http.MethodPatch, url, accessToken, orgUUID, CCRBetaHeader, payload)
	if err != nil {
		logger.DebugCF("teleport", "update title request error", map[string]any{"error": err.Error()})
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		logger.DebugCF("teleport", "successfully updated title", map[string]any{
			"session_id": sessionID,
			"title":      title,
		})
		return true
	}

	rawBody, _ := io.ReadAll(resp.Body)
	logger.DebugCF("teleport", "update title failed", map[string]any{
		"status":   resp.StatusCode,
		"response": string(rawBody),
	})
	return false
}

// TeleportFromSessionsAPI fetches session data from the session ingress API.
// Mirrors teleportFromSessionsAPI() in src/utils/teleport.tsx.
// Returns messages and branch information for session resume.
func TeleportFromSessionsAPI(ctx context.Context, baseURL, sessionID, accessToken, orgUUID string) ([]Message, string, error) {
	logger.DebugCF("teleport", "fetching session data", map[string]any{
		"session_id": sessionID,
	})

	session, err := FetchSession(ctx, baseURL, sessionID, accessToken, orgUUID)
	if err != nil {
		return nil, "", fmt.Errorf("teleport: fetch session: %w", err)
	}

	branch := getBranchFromSession(session)

	logger.DebugCF("teleport", "fetched session", map[string]any{
		"session_id": sessionID,
		"title":      session.Title,
		"branch":     branch,
	})

	return []Message{}, branch, nil
}

// PollRemoteSessionEventsResponse is a simplified response type for poll operations.
type PollRemoteSessionEventsResponse struct {
	NewEvents     []interface{} `json:"new_events"`
	LastEventID   string        `json:"last_event_id,omitempty"`
	Branch        string        `json:"branch,omitempty"`
	SessionStatus SessionStatus `json:"session_status,omitempty"`
}

// PollRemoteSessionEvents polls remote session events from the Sessions API.
// Mirrors pollRemoteSessionEvents() in src/utils/teleport.tsx.
func PollRemoteSessionEvents(ctx context.Context, baseURL, sessionID, accessToken, orgUUID string, afterID string) (*PollRemoteSessionEventsResponse, error) {
	eventsURL := fmt.Sprintf("%s/v1/sessions/%s/events", strings.TrimRight(baseURL, "/"), sessionID)
	if afterID != "" {
		eventsURL += "?after_id=" + afterID
	}

	resp, err := makeTeleportRequest(ctx, http.MethodGet, eventsURL, accessToken, orgUUID, CCRBetaHeader, nil)
	if err != nil {
		return nil, err
	}
	raw, err := readTeleportResponse(resp)
	if err != nil {
		return nil, err
	}

	type eventsResponse struct {
		Data    []interface{} `json:"data"`
		HasMore bool          `json:"has_more"`
		LastID  string        `json:"last_id"`
	}
	var parsed eventsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("teleport: decode events response: %w", err)
	}

	result := &PollRemoteSessionEventsResponse{
		NewEvents:   parsed.Data,
		LastEventID: parsed.LastID,
	}

	session, err := FetchSession(ctx, baseURL, sessionID, accessToken, orgUUID)
	if err != nil {
		logger.DebugCF("teleport", "failed to fetch session metadata for poll", map[string]any{
			"error": err.Error(),
		})
	} else {
		result.Branch = getBranchFromSession(session)
		result.SessionStatus = session.SessionStatus
	}

	return result, nil
}

// ArchiveRemoteSession archives a remote session.
// Mirrors archiveRemoteSession() in src/utils/teleport.tsx.
func ArchiveRemoteSession(ctx context.Context, baseURL, sessionID, accessToken, orgUUID string) {
	url := fmt.Sprintf("%s/v1/sessions/%s/archive", strings.TrimRight(baseURL, "/"), sessionID)
	resp, err := makeTeleportRequest(ctx, http.MethodPost, url, accessToken, orgUUID, CCRBetaHeader, []byte("{}"))
	if err != nil {
		logger.DebugCF("teleport", "archive session error", map[string]any{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 409 {
		logger.DebugCF("teleport", "archived session", map[string]any{
			"session_id": sessionID,
		})
	} else {
		raw, _ := io.ReadAll(resp.Body)
		logger.DebugCF("teleport", "archive session failed", map[string]any{
			"session_id": sessionID,
			"status":     resp.StatusCode,
			"response":   string(raw),
		})
	}
}
