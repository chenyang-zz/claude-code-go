package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

// CCRClient sends messages to a remote CCR session via HTTP POST.
// It is the write-path counterpart to the read-only EventStream used by
// LifecycleManager and SubscriptionManager.
type CCRClient struct {
	client    *http.Client
	endpoint  string
	sessionID string
	headers   map[string]string

	// tokenProvider supplies dynamic authentication tokens. When a request
	// returns 401, the client refreshes the token and retries once.
	tokenProvider TokenProvider

	// sendCount tracks the total number of successful sends.
	sendCount atomic.Int64
	// lastSendTime stores the timestamp of the most recent successful send.
	lastSendTime atomic.Pointer[time.Time]
}

// CCROption configures a CCRClient.
type CCROption func(*CCRClient)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) CCROption {
	return func(c *CCRClient) {
		c.client = client
	}
}

// WithHeader adds a custom HTTP header.
func WithHeader(key, value string) CCROption {
	return func(c *CCRClient) {
		c.headers[key] = value
	}
}

// WithTokenProvider sets a dynamic token provider for automatic 401 recovery.
func WithTokenProvider(provider TokenProvider) CCROption {
	return func(c *CCRClient) {
		c.tokenProvider = provider
	}
}

// NewCCRClient creates a CCRClient for sending messages to the given endpoint.
// endpoint should be a fully-qualified URL (e.g. https://host/sessions/{id}/messages).
func NewCCRClient(endpoint, sessionID string, opts ...CCROption) *CCRClient {
	c := &CCRClient{
		client:    &http.Client{Timeout: 30 * time.Second},
		endpoint:  strings.TrimSpace(endpoint),
		sessionID: strings.TrimSpace(sessionID),
		headers:   make(map[string]string),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Send posts raw data to the remote endpoint using a default 30-second timeout.
func (c *CCRClient) Send(data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return c.SendWithContext(ctx, data)
}

// SendWithContext posts raw data with the given context.
func (c *CCRClient) SendWithContext(ctx context.Context, data []byte) error {
	if c == nil {
		return fmt.Errorf("ccr client is nil")
	}
	if c.endpoint == "" {
		return fmt.Errorf("ccr client endpoint not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	c.applyCurrentAuthHeader(req)

	logger.DebugCF("remote_ccr", "sending message", map[string]any{
		"endpoint": c.endpoint,
		"size":     len(data),
	})

	resp, err := c.doRequestWithAuthRetry(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		logger.WarnCF("remote_ccr", "message send rejected", map[string]any{
			"status":   resp.StatusCode,
			"endpoint": c.endpoint,
			"body":     string(body),
		})
		return classifyHTTPError(resp.StatusCode, string(body))
	}

	logger.DebugCF("remote_ccr", "message sent successfully", map[string]any{
		"endpoint": c.endpoint,
		"status":   resp.StatusCode,
	})
	c.sendCount.Add(1)
	now := time.Now()
	c.lastSendTime.Store(&now)
	return nil
}

// SendUserMessage serializes and sends a user message to the remote session.
// If the CCRClient was created with a non-empty sessionID, it is injected into
// the message when the message itself does not already carry one.
func (c *CCRClient) SendUserMessage(ctx context.Context, msg sdk.User) error {
	if c.sessionID != "" && msg.SessionID == "" {
		msg.SessionID = c.sessionID
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal user message: %w", err)
	}
	return c.SendWithContext(ctx, data)
}

// SendControlResponse serializes and sends a control response to the remote session.
func (c *CCRClient) SendControlResponse(ctx context.Context, resp sdk.ControlResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal control response: %w", err)
	}
	return c.SendWithContext(ctx, data)
}

// ---------------------------------------------------------------------------
// 401 auth recovery
// ---------------------------------------------------------------------------

// applyCurrentAuthHeader sets the Authorization header from the token provider
// if one is configured. This is called before every request so that token
// updates are picked up without restarting the client.
func (c *CCRClient) applyCurrentAuthHeader(req *http.Request) {
	if c.tokenProvider == nil {
		return
	}
	applyAuthHeader(req, c.tokenProvider.Token())
}

// doRequestWithAuthRetry executes an HTTP request and handles 401 responses by
// refreshing the authentication token and retrying once. If the refreshed
// token is the same as the old one, or if no token provider is configured,
// the original 401 response is returned without modification.
func (c *CCRClient) doRequestWithAuthRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, classifySendError(err)
	}

	if resp.StatusCode != http.StatusUnauthorized || c.tokenProvider == nil {
		return resp, nil
	}

	// 401 detected — try to refresh the token.
	oldToken := c.tokenProvider.Token()
	newToken, refreshErr := c.tokenProvider.Refresh()
	if refreshErr != nil || newToken == "" || newToken == oldToken {
		// Refresh failed or token unchanged; return the original 401.
		return resp, nil
	}

	// Token changed; close the old response and retry with the new token.
	resp.Body.Close()

	// Clone the request so headers can be updated without affecting the original.
	retryReq := req.Clone(ctx)

	// Obtain a fresh body for the retry. http.NewRequestWithContext sets
	// GetBody for rewindable readers (e.g. *bytes.Reader), but wraps the
	// raw Body in io.NopCloser which does not implement io.Seeker.
	if req.GetBody != nil {
		newBody, err := req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("get body for retry: %w", err)
		}
		retryReq.Body = newBody
	} else if req.Body != nil {
		if seeker, ok := req.Body.(io.Seeker); ok {
			if _, err := seeker.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("rewind request body for retry: %w", err)
			}
		}
	}
	applyAuthHeader(retryReq, newToken)

	logger.DebugCF("remote_ccr", "retrying request after token refresh", map[string]any{
		"endpoint": c.endpoint,
		"old_token_prefix": oldToken[:min(len(oldToken), 8)],
		"new_token_prefix": newToken[:min(len(newToken), 8)],
	})

	return c.client.Do(retryReq)
}

// ---------------------------------------------------------------------------
// Error classification
// ---------------------------------------------------------------------------

// SendErrorKind classifies HTTP send failures.
type SendErrorKind int

const (
	// SendErrorNetwork indicates a transport-level failure (DNS, connection reset, etc.).
	SendErrorNetwork SendErrorKind = iota
	// SendErrorAuth indicates an authentication failure (401/403).
	SendErrorAuth
	// SendErrorTimeout indicates the request timed out.
	SendErrorTimeout
	// SendErrorRateLimit indicates rate limiting (429).
	SendErrorRateLimit
	// SendErrorServer indicates a server error (5xx).
	SendErrorServer
	// SendErrorOther indicates an unclassified failure.
	SendErrorOther
)

func (k SendErrorKind) String() string {
	switch k {
	case SendErrorNetwork:
		return "network"
	case SendErrorAuth:
		return "auth"
	case SendErrorTimeout:
		return "timeout"
	case SendErrorRateLimit:
		return "rate_limit"
	case SendErrorServer:
		return "server"
	default:
		return "other"
	}
}

// SendError wraps a classified send failure.
type SendError struct {
	Kind    SendErrorKind
	Message string
	Status  int
}

// Error implements the error interface.
func (e *SendError) Error() string {
	return fmt.Sprintf("ccr send error [%s]: %s", e.Kind.String(), e.Message)
}

// IsRetryable reports whether the send error is retryable.
func (e *SendError) IsRetryable() bool {
	switch e.Kind {
	case SendErrorNetwork, SendErrorTimeout, SendErrorRateLimit, SendErrorServer:
		return true
	case SendErrorAuth, SendErrorOther:
		return false
	default:
		return false
	}
}

// IsSendError reports whether err is a *SendError and returns it.
func IsSendError(err error) (*SendError, bool) {
	var se *SendError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}

func classifySendError(err error) error {
	if err == nil {
		return nil
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return &SendError{Kind: SendErrorTimeout, Message: err.Error()}
		}
		return &SendError{Kind: SendErrorNetwork, Message: err.Error()}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return &SendError{Kind: SendErrorTimeout, Message: err.Error()}
	}

	return &SendError{Kind: SendErrorNetwork, Message: err.Error()}
}

func classifyHTTPError(status int, body string) error {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &SendError{Kind: SendErrorAuth, Message: body, Status: status}
	case status == http.StatusTooManyRequests:
		return &SendError{Kind: SendErrorRateLimit, Message: body, Status: status}
	case status >= http.StatusInternalServerError:
		return &SendError{Kind: SendErrorServer, Message: body, Status: status}
	default:
		return &SendError{Kind: SendErrorOther, Message: body, Status: status}
	}
}

// AuthState returns the current authentication state if a token provider that
// supports state observation is configured. Otherwise returns a zero AuthState.
func (c *CCRClient) AuthState() AuthState {
	if c == nil {
		return AuthState{}
	}
	if asp, ok := c.tokenProvider.(AuthStateProvider); ok {
		return asp.AuthState()
	}
	return AuthState{}
}

// SendCount returns the total number of successful sends performed by this client.
func (c *CCRClient) SendCount() int64 {
	if c == nil {
		return 0
	}
	return c.sendCount.Load()
}

// LastSendTime returns the timestamp of the most recent successful send,
// or the zero time if no send has succeeded yet.
func (c *CCRClient) LastSendTime() time.Time {
	if c == nil {
		return time.Time{}
	}
	if t := c.lastSendTime.Load(); t != nil {
		return *t
	}
	return time.Time{}
}

// DeriveEndpoint derives the HTTP POST endpoint from remote session config.
// Priority:
//   1. CLAUDE_CODE_REMOTE_POST_URL environment variable
//   2. StreamURL with ws scheme replaced by http/s
func DeriveEndpoint(session coreconfig.RemoteSessionConfig) string {
	if url := os.Getenv("CLAUDE_CODE_REMOTE_POST_URL"); url != "" {
		return strings.TrimSpace(url)
	}
	if session.StreamURL != "" {
		url := session.StreamURL
		url = strings.Replace(url, "wss://", "https://", 1)
		url = strings.Replace(url, "ws://", "http://", 1)
		return url
	}
	return ""
}
