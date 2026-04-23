package remote

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveAuthToken resolves the remote session ingress authentication token from
// known sources, following the same priority order as the TypeScript host:
//
//  1. CLAUDE_CODE_SESSION_ACCESS_TOKEN environment variable
//  2. CLAUDE_SESSION_INGRESS_TOKEN_FILE environment variable (file path)
//  3. ~/.claude/remote/.session_ingress_token (well-known path)
//
// An empty string is returned when no token source is available.
func ResolveAuthToken() string {
	if token := os.Getenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN"); token != "" {
		return strings.TrimSpace(token)
	}

	if path := os.Getenv("CLAUDE_SESSION_INGRESS_TOKEN_FILE"); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data))
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".claude", "remote", ".session_ingress_token")
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data))
		}
	}

	return ""
}

// AuthHeaders builds the standard authentication header map for remote session
// ingress requests. When a token is resolved, it is sent as a Bearer token in
// the Authorization header.
func AuthHeaders() map[string]string {
	headers := make(map[string]string)
	if token := ResolveAuthToken(); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return headers
}
