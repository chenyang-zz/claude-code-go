package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// CreateSessionParams holds the request parameters for creating a direct-connect session.
type CreateSessionParams struct {
	ServerURL                 string
	AuthToken                 string
	CWD                       string
	DangerouslySkipPermissions bool
}

// CreateSessionError is returned when creating a direct-connect session fails.
type CreateSessionError struct {
	Message string
}

func (e *CreateSessionError) Error() string {
	return e.Message
}

// CreateDirectConnectSession creates a session on a direct-connect server.
//
// It POSTs to ${serverURL}/sessions with authentication and session parameters,
// validates the response, and returns a DirectConnectConfig.
func CreateDirectConnectSession(params CreateSessionParams) (*DirectConnectConfig, string, error) {
	headers := map[string]string{
		"content-type": "application/json",
	}
	if params.AuthToken != "" {
		headers["authorization"] = "Bearer " + params.AuthToken
	}

	body := map[string]any{
		"cwd": params.CWD,
	}
	if params.DangerouslySkipPermissions {
		body["dangerously_skip_permissions"] = true
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "", &CreateSessionError{
			Message: fmt.Sprintf("marshal request body: %v", err),
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	url := params.ServerURL + "/sessions"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, "", &CreateSessionError{
			Message: fmt.Sprintf("create request: %v", err),
		}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	logger.DebugCF("server_session", "creating direct-connect session", map[string]any{
		"server_url": params.ServerURL,
	})

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", &CreateSessionError{
			Message: fmt.Sprintf("failed to connect to server at %s: %v", params.ServerURL, err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, "", &CreateSessionError{
			Message: fmt.Sprintf("failed to create session: %d %s", resp.StatusCode, string(respBody)),
		}
	}

	var connectResp ConnectResponse
	if err := json.NewDecoder(resp.Body).Decode(&connectResp); err != nil {
		return nil, "", &CreateSessionError{
			Message: fmt.Sprintf("invalid session response: %v", err),
		}
	}

	config := &DirectConnectConfig{
		ServerURL: params.ServerURL,
		SessionID: connectResp.SessionID,
		WsURL:     connectResp.WsURL,
		AuthToken: params.AuthToken,
	}

	logger.DebugCF("server_session", "session created", map[string]any{
		"session_id": config.SessionID,
	})

	return config, connectResp.WorkDir, nil
}
