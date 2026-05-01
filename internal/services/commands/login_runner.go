package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	platformconfig "github.com/sheepzhao/claude-code-go/internal/platform/config"
	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// LoginRunnerDeps configures the production /login OAuth runner. All fields
// have sensible defaults: leaving them zero produces a runner that talks to
// the real Anthropic OAuth endpoints, reads from stdin, and prints to
// stdout.
type LoginRunnerDeps struct {
	// HomeDir is the user's home directory. Required.
	HomeDir string
	// ProjectDir is the current project root, used by SettingsWriter for
	// project-scoped writes. Optional.
	ProjectDir string

	// Stdin is the source of user input for the manual paste path. Defaults
	// to os.Stdin.
	Stdin io.Reader
	// Stdout is the writer the URLs and prompts are emitted to. Defaults to
	// os.Stdout.
	Stdout io.Writer

	// OpenBrowser opens the supplied URL in the user's default browser.
	// Defaults to a runtime-specific exec call (open / xdg-open / start).
	OpenBrowser func(url string) error

	// NewService, when non-nil, overrides the OAuth service constructor;
	// useful for tests.
	NewService func() *oauth.OAuthService

	// FlowOptions is forwarded verbatim to OAuthService.StartOAuthFlow.
	FlowOptions oauth.StartOAuthFlowOptions
}

// NewLoginRunner builds a LoginRunFunc that performs the real interactive
// OAuth login. The returned function is suitable for use as
// LoginCommand.Login when the application is wired with OAuth support.
func NewLoginRunner(deps LoginRunnerDeps) (LoginRunFunc, error) {
	if strings.TrimSpace(deps.HomeDir) == "" {
		return nil, fmt.Errorf("login runner: HomeDir is required")
	}
	stdin := deps.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := deps.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	openBrowser := deps.OpenBrowser
	if openBrowser == nil {
		openBrowser = defaultOpenBrowser
	}
	newService := deps.NewService
	if newService == nil {
		newService = oauth.NewOAuthService
	}

	credentials, err := oauth.NewOAuthCredentialStore(deps.HomeDir)
	if err != nil {
		return nil, err
	}
	settingsWriter := platformconfig.NewSettingsWriter(deps.HomeDir, deps.ProjectDir)

	return func(ctx context.Context) (*LoginOutcome, error) {
		service := newService()
		defer func() { _ = service.Cleanup() }()

		manualPasteCh := make(chan struct{}, 1)

		authURLHandler := func(manualURL, automaticURL string) error {
			fmt.Fprintln(stdout, "Sign in to your Anthropic account.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Open this URL in your browser to continue:")
			fmt.Fprintf(stdout, "  %s\n", automaticURL)
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "If the browser flow doesn't complete, open this URL instead and paste the code below:")
			fmt.Fprintf(stdout, "  %s\n", manualURL)
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Waiting for authorization (or paste a code and press Enter)...")
			if err := openBrowser(automaticURL); err != nil {
				logger.WarnCF("commands", "failed to open browser for OAuth login", map[string]any{
					"error": err.Error(),
				})
			}
			go readManualCodeFromStdin(stdin, service, manualPasteCh)
			return nil
		}

		flowOpts := deps.FlowOptions
		flowOpts.SkipBrowserOpen = true // we handle the browser open ourselves

		tokens, err := service.StartOAuthFlow(ctx, flowOpts, authURLHandler)
		// Drain the manual-paste reader if it is still alive.
		select {
		case <-manualPasteCh:
		default:
		}
		if err != nil {
			return nil, err
		}

		decision, saveErr := credentials.Save(tokens)
		if saveErr != nil {
			return nil, fmt.Errorf("login runner: persist credentials: %w", saveErr)
		}

		outcome := &LoginOutcome{
			ScopeCount: len(tokens.Scopes),
		}
		if tokens.TokenAccount != nil {
			outcome.Email = tokens.TokenAccount.EmailAddress
			outcome.OrganizationUUID = tokens.TokenAccount.OrganizationUUID
		}
		if tokens.Profile != nil {
			if tokens.Profile.Account.Email != "" {
				outcome.Email = tokens.Profile.Account.Email
			}
			if tokens.Profile.Account.DisplayName != "" {
				// We use display name as the human-readable account label
				// when no organization label is available; org name itself
				// is not part of the profile envelope.
			}
		}
		outcome.SubscriptionType = string(tokens.SubscriptionType)
		if decision.Persisted {
			outcome.CredentialsPath = credentials.Path()
		} else {
			outcome.Skipped = true
			outcome.SkipReason = decision.SkipReason
		}

		// Persist account metadata into ~/.claude/settings.json under
		// `oauthAccount.*`. Failures are non-fatal: tokens are already on
		// disk, and the metadata is decorative.
		writeOAuthAccountMetadata(ctx, settingsWriter, outcome)
		return outcome, nil
	}, nil
}

// writeOAuthAccountMetadata writes the four OAuthAccountConfig fields back to
// the user-scope settings.json. It logs and swallows errors because the
// /login flow has already succeeded by the time this runs.
func writeOAuthAccountMetadata(ctx context.Context, writer *platformconfig.SettingsWriter, outcome *LoginOutcome) {
	if writer == nil || outcome == nil {
		return
	}
	updates := map[string]string{
		"oauthAccount.accountUuid":      "",
		"oauthAccount.emailAddress":     outcome.Email,
		"oauthAccount.organizationUuid": outcome.OrganizationUUID,
		"oauthAccount.organizationName": outcome.OrganizationName,
	}
	for key, value := range updates {
		if value == "" {
			continue
		}
		if err := writer.Set(ctx, "user", key, value); err != nil {
			logger.WarnCF("commands", "failed to update oauthAccount metadata", map[string]any{
				"key":   key,
				"error": err.Error(),
			})
		}
	}
}

// readManualCodeFromStdin reads one line from the supplied reader and feeds
// it back to the OAuthService as a manual auth code. Empty input or read
// errors are silently dropped; any submission error is logged but does not
// block the listener-based path.
func readManualCodeFromStdin(stdin io.Reader, service *oauth.OAuthService, doneCh chan<- struct{}) {
	defer func() {
		select {
		case doneCh <- struct{}{}:
		default:
		}
	}()
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		return
	}
	pasted := strings.TrimSpace(scanner.Text())
	if pasted == "" {
		return
	}
	// Manual codes from the hosted page are typically `<code>#<state>`. We
	// accept either form: with or without a trailing state.
	state := ""
	code := pasted
	if idx := strings.LastIndex(pasted, "#"); idx >= 0 {
		code = pasted[:idx]
		state = pasted[idx+1:]
	}
	if err := service.HandleManualAuthCodeInput(state, code); err != nil {
		logger.DebugCF("commands", "manual auth code submission rejected", map[string]any{
			"error": err.Error(),
		})
	}
}

// defaultOpenBrowser invokes the platform-specific command that opens a URL
// in the user's default browser. Errors are returned to the caller.
func defaultOpenBrowser(target string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("open browser: empty URL")
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", "", target).Start()
	default: // linux, freebsd, ...
		return exec.Command("xdg-open", target).Start()
	}
}
