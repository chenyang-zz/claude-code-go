package anthropic

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GoogleAuthenticator abstracts the token source used for Vertex AI authentication.
type GoogleAuthenticator interface {
	// GetToken returns a valid OAuth2 access token for the cloud-platform scope.
	GetToken(ctx context.Context) (string, error)
}

// DefaultGoogleAuthenticator attempts to obtain a Google access token using
// the following strategy:
//
//  1. GOOGLE_ACCESS_TOKEN environment variable (highest priority)
//  2. gcloud auth print-access-token (if gcloud CLI is available)
//
// For production use, users should run 'gcloud auth application-default login'
// before starting the application or set GOOGLE_ACCESS_TOKEN explicitly.
type DefaultGoogleAuthenticator struct{}

// GetToken obtains a Google access token.
func (a *DefaultGoogleAuthenticator) GetToken(ctx context.Context) (string, error) {
	// 1. Prefer explicit environment variable.
	if token := os.Getenv("GOOGLE_ACCESS_TOKEN"); token != "" {
		return token, nil
	}

	// 2. Fall back to gcloud CLI.
	cmd := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf(
			"no google access token available (set GOOGLE_ACCESS_TOKEN or run 'gcloud auth application-default login'): %w",
			err,
		)
	}
	return strings.TrimSpace(string(out)), nil
}

// noopGoogleAuthenticator returns an empty token. It is used when
// CLAUDE_CODE_SKIP_VERTEX_AUTH is set (testing / proxy scenarios).
type noopGoogleAuthenticator struct{}

func (a *noopGoogleAuthenticator) GetToken(_ context.Context) (string, error) {
	return "", nil
}

// newGoogleAuthenticator builds the appropriate authenticator based on environment.
// When skipAuth is true it returns a noop authenticator.
// Otherwise it returns the default authenticator.
func newGoogleAuthenticator(skipAuth bool) GoogleAuthenticator {
	if skipAuth {
		return &noopGoogleAuthenticator{}
	}
	return &DefaultGoogleAuthenticator{}
}

// getVertexProjectID returns the GCP project ID to use for Vertex AI requests.
// It follows the same fallback order as the TS implementation:
// 1. GCLOUD_PROJECT or GOOGLE_CLOUD_PROJECT environment variables
// 2. ANTHROPIC_VERTEX_PROJECT_ID as last-resort fallback
func getVertexProjectID() string {
	for _, key := range []string{
		"GCLOUD_PROJECT",
		"GOOGLE_CLOUD_PROJECT",
		"gcloud_project",
		"google_cloud_project",
	} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")
}
