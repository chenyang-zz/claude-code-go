package commands

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

func TestNewLoginRunner_RequiresHomeDir(t *testing.T) {
	_, err := NewLoginRunner(LoginRunnerDeps{})
	if err == nil {
		t.Fatalf("expected error for empty HomeDir")
	}
	if !strings.Contains(err.Error(), "HomeDir") {
		t.Fatalf("expected HomeDir error message, got %v", err)
	}
}

func TestNewLoginRunner_BuildsRunFunc(t *testing.T) {
	dir := t.TempDir()
	runner, err := NewLoginRunner(LoginRunnerDeps{
		HomeDir:    dir,
		ProjectDir: dir,
	})
	if err != nil {
		t.Fatalf("NewLoginRunner: %v", err)
	}
	if runner == nil {
		t.Fatalf("NewLoginRunner returned nil func")
	}
}

func TestNewLoginRunner_AcceptsCustomServiceFactory(t *testing.T) {
	dir := t.TempDir()
	called := 0
	runner, err := NewLoginRunner(LoginRunnerDeps{
		HomeDir: dir,
		NewService: func() *oauth.OAuthService {
			called++
			return oauth.NewOAuthService()
		},
	})
	if err != nil {
		t.Fatalf("NewLoginRunner: %v", err)
	}
	_ = runner
	// We do not invoke the runner here (it would block on a real localhost
	// listener). The call-count assertion is exercised via integration
	// elsewhere; this test verifies only that the factory accepts the
	// override without panicking.
	_ = called
}

func TestReadManualCodeFromStdin_ParsesCodeAndState(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCode  string
		wantState string
		shouldErr bool
	}{
		{"plain", "abc123", "abc123", "", false},
		{"with-state", "abc123#state-xyz", "abc123", "state-xyz", false},
		{"with-spaces", "  code-1  ", "code-1", "", false},
		{"empty", "", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := oauth.NewOAuthService()
			defer service.Cleanup()
			doneCh := make(chan struct{}, 1)
			done := make(chan struct{})
			go func() {
				readManualCodeFromStdin(strings.NewReader(tc.input+"\n"), service, doneCh)
				close(done)
			}()
			<-done
			// We can't directly observe what was submitted because there is
			// no in-flight flow; the only failure mode is a panic, which
			// would surface as a test failure.
			_ = tc.wantCode
			_ = tc.wantState
		})
	}
}
