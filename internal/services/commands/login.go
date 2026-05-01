package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// LoginOutcome captures the result of a successful interactive Anthropic
// account login. It is the observable surface returned by LoginRunFunc;
// LoginCommand only formats it into a console-ready string.
type LoginOutcome struct {
	// Email is the authenticated user's email address (from the profile or
	// the token-account block). Empty when the upstream did not return one.
	Email string
	// OrganizationName is the organization display name; falls back to the
	// organization UUID when the name is unavailable.
	OrganizationName string
	// OrganizationUUID is the canonical organization identifier.
	OrganizationUUID string
	// SubscriptionType is the mapped tier (e.g. "max", "pro"); empty when
	// the profile fetch did not run or the tier is unknown.
	SubscriptionType string
	// ScopeCount is the number of OAuth scopes granted by the upstream.
	ScopeCount int
	// CredentialsPath is the absolute path to the file the credentials were
	// written to. Empty when persistence was skipped.
	CredentialsPath string
	// Skipped reports whether credentials were not persisted (e.g. because
	// the granted scopes did not include user:inference). When true the
	// caller should treat the login as best-effort metadata only.
	Skipped bool
	// SkipReason explains why credentials were not persisted; only set when
	// Skipped is true.
	SkipReason string
}

// LoginRunFunc executes a full Anthropic account OAuth login flow:
// orchestrating the listener, exchange, profile fetch, and persistence. A
// nil LoginRunFunc on a LoginCommand triggers the fallback-text path so
// existing tests and bootstrap configurations without OAuth wiring continue
// to work.
type LoginRunFunc func(ctx context.Context) (*LoginOutcome, error)

// LoginCommand renders the /login slash command. When Login is non-nil and
// no API key/auth token is configured the command runs the real OAuth flow
// and reports the resulting account; otherwise it falls back to the stable
// occupier text.
type LoginCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
	// Login, when non-nil, executes the real interactive OAuth flow.
	Login LoginRunFunc
}

// Metadata returns the canonical slash descriptor for /login.
func (c LoginCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "login",
		Description: "Sign in with your Anthropic account",
		Usage:       "/login",
	}
}

// Execute reports the stable authentication guidance supported by the
// current Go host. When OAuth wiring is available it runs the interactive
// flow and returns a structured login summary.
func (c LoginCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = args

	usingAPIKey := strings.TrimSpace(c.Config.APIKey) != ""
	usingAuthToken := strings.TrimSpace(c.Config.AuthToken) != ""
	logger.DebugCF("commands", "rendered login command output", map[string]any{
		"using_api_key":    usingAPIKey,
		"using_auth_token": usingAuthToken,
		"oauth_available":  c.Login != nil,
	})

	if usingAPIKey {
		return command.Result{
			Output: "Claude Code Go is using configured API key authentication. Interactive account switching is not supported yet.",
		}, nil
	}
	if usingAuthToken {
		return command.Result{
			Output: "Claude Code Go is using configured auth token authentication. Interactive account switching is not supported yet.",
		}, nil
	}

	if c.Login == nil {
		return command.Result{
			Output: "Interactive Anthropic account login is not supported in Claude Code Go yet. Configure an API key or auth token in settings or environment variables instead.",
		}, nil
	}

	outcome, err := c.Login(ctx)
	if err != nil {
		return command.Result{}, fmt.Errorf("/login: %w", err)
	}
	if outcome == nil {
		return command.Result{}, fmt.Errorf("/login: login runner returned a nil outcome")
	}
	return command.Result{
		Output: formatLoginOutcome(outcome),
	}, nil
}

// formatLoginOutcome renders a LoginOutcome as a multi-line console summary
// suitable for the /login command output.
func formatLoginOutcome(outcome *LoginOutcome) string {
	var b strings.Builder
	b.WriteString("Logged in to your Anthropic account.\n")
	if outcome.Email != "" {
		fmt.Fprintf(&b, "  Email: %s\n", outcome.Email)
	}
	if outcome.OrganizationName != "" {
		fmt.Fprintf(&b, "  Organization: %s\n", outcome.OrganizationName)
	} else if outcome.OrganizationUUID != "" {
		fmt.Fprintf(&b, "  Organization: %s\n", outcome.OrganizationUUID)
	}
	if outcome.SubscriptionType != "" {
		fmt.Fprintf(&b, "  Subscription: %s\n", outcome.SubscriptionType)
	}
	if outcome.ScopeCount > 0 {
		fmt.Fprintf(&b, "  Scopes granted: %d\n", outcome.ScopeCount)
	}
	if outcome.Skipped {
		fmt.Fprintf(&b, "  Credentials NOT persisted: %s\n", outcome.SkipReason)
	} else if outcome.CredentialsPath != "" {
		fmt.Fprintf(&b, "  Credentials saved to: %s\n", outcome.CredentialsPath)
	}
	return strings.TrimRight(b.String(), "\n")
}
