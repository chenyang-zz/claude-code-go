package remote

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// TokenProvider provides dynamic authentication token retrieval.
// In remote mode the token is injected by the parent process via environment
// variables; when it expires the parent may update the env var and
// TokenProvider re-reads it on demand.
type TokenProvider interface {
	// Token returns the current cached token.
	Token() string
	// Refresh re-resolves the token from its sources and updates the cached
	// value. Returns the new token and a nil error on success. If no token is
	// available, returns an empty string and a non-nil error.
	Refresh() (string, error)
}

// AuthState holds observable metadata about the current token.
type AuthState struct {
	// Token is the current token value (empty if none).
	Token string
	// Source describes where the token came from (e.g. "env", "file", "well-known").
	Source string
	// RefreshedAt is the timestamp of the last successful refresh.
	RefreshedAt time.Time
	// RefreshCount is the total number of refresh calls.
	RefreshCount int
}

// AuthStateProvider exposes authentication state for observability.
type AuthStateProvider interface {
	// AuthState returns the current authentication state.
	AuthState() AuthState
}

// EnvTokenProvider resolves tokens from environment variables and well-known
// file paths, re-reading on each Refresh call so that parent-process token
// updates are picked up without restarting the CLI.
type EnvTokenProvider struct {
	mu           sync.RWMutex
	token        string
	source       string
	refreshedAt  time.Time
	refreshCount int
}

// NewEnvTokenProvider creates a provider that resolves the initial token from
// the standard auth sources and can be refreshed later.
func NewEnvTokenProvider() *EnvTokenProvider {
	token, source := resolveAuthTokenWithSource()
	return &EnvTokenProvider{
		token:  token,
		source: source,
	}
}

// Token returns the current cached token.
func (p *EnvTokenProvider) Token() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.token
}

// Refresh re-resolves the token from its sources. If the token source has been
// updated (e.g. by the parent CCR process changing the environment variable),
// the new value is cached and returned.
func (p *EnvTokenProvider) Refresh() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.refreshCount++
	newToken, newSource := resolveAuthTokenWithSource()
	p.refreshedAt = time.Now()

	if newToken == "" {
		logger.WarnCF("remote_auth", "token refresh failed: no token available", nil)
		return "", errors.New("no auth token available")
	}

	changed := newToken != p.token
	p.token = newToken
	p.source = newSource
	logger.DebugCF("remote_auth", "token refreshed", map[string]any{
		"source":        newSource,
		"changed":       changed,
		"refresh_count": p.refreshCount,
	})
	return newToken, nil
}

// AuthState returns the current observable authentication state.
func (p *EnvTokenProvider) AuthState() AuthState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return AuthState{
		Token:        p.token,
		Source:       p.source,
		RefreshedAt:  p.refreshedAt,
		RefreshCount: p.refreshCount,
	}
}

// resolveAuthTokenWithSource resolves the remote session ingress authentication
// token from known sources and reports which source provided it.
//
// Priority order:
//  1. CLAUDE_CODE_SESSION_ACCESS_TOKEN environment variable
//  2. CLAUDE_SESSION_INGRESS_TOKEN_FILE environment variable (file path)
//  3. ~/.claude/remote/.session_ingress_token (well-known path)
func resolveAuthTokenWithSource() (string, string) {
	if token := os.Getenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN"); token != "" {
		return strings.TrimSpace(token), "env"
	}

	if path := os.Getenv("CLAUDE_SESSION_INGRESS_TOKEN_FILE"); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data)), "file"
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".claude", "remote", ".session_ingress_token")
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data)), "well-known"
		}
	}

	return "", ""
}

// applyAuthHeader sets or updates the Authorization header on req using the
// provided token. It handles both Bearer tokens and session keys (sk-ant-sid)
// which use Cookie auth.
func applyAuthHeader(req *http.Request, token string) {
	if token == "" {
		return
	}
	if strings.HasPrefix(token, "sk-ant-sid") {
		req.Header.Set("Cookie", "sessionKey="+token)
		if orgUuid := os.Getenv("CLAUDE_CODE_ORGANIZATION_UUID"); orgUuid != "" {
			req.Header.Set("X-Organization-Uuid", orgUuid)
		}
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
