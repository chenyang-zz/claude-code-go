package commands

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	fileedit "github.com/sheepzhao/claude-code-go/internal/services/tools/file_edit"
	fileread "github.com/sheepzhao/claude-code-go/internal/services/tools/file_read"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/glob"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/grep"
)

// TestStatusCommandMetadata verifies /status exposes stable metadata.
func TestStatusCommandMetadata(t *testing.T) {
	meta := StatusCommand{}.Metadata()
	if meta.Name != "status" {
		t.Fatalf("Metadata().Name = %q, want status", meta.Name)
	}
	if meta.Description != "Show Claude Code status including version, model, account, API connectivity, and tool statuses" {
		t.Fatalf("Metadata().Description = %q, want stable status description", meta.Description)
	}
	if meta.Usage != "/status" {
		t.Fatalf("Metadata().Usage = %q, want /status", meta.Usage)
	}
}

type stubStatusProbe struct {
	result APIConnectivityProbeResult
}

func (p stubStatusProbe) Probe(context.Context, coreconfig.Config) APIConnectivityProbeResult {
	return p.result
}

type statusStubFileInfo struct{}

func (statusStubFileInfo) Name() string       { return "" }
func (statusStubFileInfo) Size() int64        { return 0 }
func (statusStubFileInfo) Mode() os.FileMode  { return 0 }
func (statusStubFileInfo) ModTime() time.Time { return time.Time{} }
func (statusStubFileInfo) IsDir() bool        { return true }
func (statusStubFileInfo) Sys() any           { return nil }

type statusStubRegularFileInfo struct{}

func (statusStubRegularFileInfo) Name() string       { return "" }
func (statusStubRegularFileInfo) Size() int64        { return 0 }
func (statusStubRegularFileInfo) Mode() os.FileMode  { return 0 }
func (statusStubRegularFileInfo) ModTime() time.Time { return time.Time{} }
func (statusStubRegularFileInfo) IsDir() bool        { return false }
func (statusStubRegularFileInfo) Sys() any           { return nil }

// TestStatusCommandExecute verifies /status reports the current Go host summary and stable fallback boundaries.
func TestStatusCommandExecute(t *testing.T) {
	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}
	toolRegistry := coretool.NewMemoryRegistry()
	for _, tool := range []coretool.Tool{
		glob.NewTool(platformfs.NewLocalFS(), policy),
		grep.NewTool(platformfs.NewLocalFS(), policy),
		fileread.NewTool(platformfs.NewLocalFS(), policy),
		fileedit.NewTool(platformfs.NewLocalFS(), policy),
	} {
		if err := toolRegistry.Register(tool); err != nil {
			t.Fatalf("Register(tool=%s) error = %v", tool.Name(), err)
		}
	}

	statFn := func(path string) (os.FileInfo, error) {
		switch path {
		case "/tmp":
			return statusStubFileInfo{}, nil
		default:
			return nil, errors.New("missing")
		}
	}

	result, err := StatusCommand{
		Config: coreconfig.Config{
			Provider:              "anthropic",
			Model:                 "claude-sonnet-4-5",
			ProjectPath:           "/repo/project",
			ApprovalMode:          "default",
			SessionDBPath:         "/tmp/sessions.db",
			LoadedSettingSources:  []string{"userSettings", "projectSettings", "localSettings"},
			APIKey:                "test-key",
			APIKeySource:          "ANTHROPIC_API_KEY",
			ProxyURL:              "http://proxy.internal:8080",
			AdditionalCACertsPath: "/etc/ssl/custom.pem",
			MTLSClientCertPath:    "/etc/ssl/client.pem",
			MTLSClientKeyPath:     "/etc/ssl/client-key.pem",
		},
		ToolRegistry: toolRegistry,
		APIProbe: stubStatusProbe{
			result: APIConnectivityProbeResult{
				Summary: "reachable (HTTP 204 from /v1/messages)",
			},
		},
		Stat: statFn,
		LookPath: func(string) (string, error) {
			return "", errors.New("missing")
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Status summary:\n- Provider: anthropic\n- API provider type: Anthropic first-party\n- Model: claude-sonnet-4-5\n- Project path: /repo/project\n- Approval mode: default\n- Session storage: /tmp/sessions.db (not created yet; parent directory exists)\n- Settings sources: User settings, Project settings, Local settings\n- Account auth: API key configured; interactive account status is not available\n- API key source: ANTHROPIC_API_KEY\n- Auth token source: not configured\n- API base URL: default\n- API base URL source: default\n- Proxy: http://proxy.internal:8080\n- Additional CA cert(s): /etc/ssl/custom.pem\n- mTLS client cert: /etc/ssl/client.pem\n- mTLS client key: /etc/ssl/client-key.pem\n- Bash sandbox: not available in Claude Code Go yet\n- MCP servers: no MCP tools registered\n- Memory files: no CLAUDE.md files detected\n- Installation health: ripgrep missing from PATH\n- API connectivity check: reachable (HTTP 204 from /v1/messages)\n- Tool status checks: 4 registered (Glob, Grep, Read, Edit)\n- Settings status UI: not available in Claude Code Go yet"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestStatusCommandExecuteWithAuthToken verifies /status treats Anthropic auth token authentication as configured.
func TestStatusCommandExecuteWithAuthToken(t *testing.T) {
	result, err := StatusCommand{
		Config: coreconfig.Config{
			Provider:        "anthropic",
			AuthToken:       "auth-token",
			AuthTokenSource: "ANTHROPIC_AUTH_TOKEN",
			ProjectPath:     "/repo/project",
		},
		APIProbe: stubStatusProbe{
			result: APIConnectivityProbeResult{
				Summary: "reachable (HTTP 401 from /v1/messages)",
			},
		},
		LookPath: func(string) (string, error) {
			return "", errors.New("missing")
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Status summary:\n- Provider: anthropic\n- API provider type: Anthropic first-party\n- Model: (not set)\n- Project path: /repo/project\n- Approval mode: (not set)\n- Session storage: not configured\n- Settings sources: none\n- Account auth: Auth token configured; interactive account status is not available\n- API key source: not configured\n- Auth token source: ANTHROPIC_AUTH_TOKEN\n- API base URL: default\n- API base URL source: default\n- Bash sandbox: not available in Claude Code Go yet\n- MCP servers: no MCP tools registered\n- Memory files: no CLAUDE.md files detected\n- Installation health: ripgrep missing from PATH\n- API connectivity check: reachable (HTTP 401 from /v1/messages)\n- Tool status checks: no tools registered\n- Settings status UI: not available in Claude Code Go yet"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestStatusCommandExecuteWithoutCredential verifies /status keeps the missing-account fallback stable.
func TestStatusCommandExecuteWithoutCredential(t *testing.T) {
	result, err := StatusCommand{
		LookPath: func(string) (string, error) {
			return "", errors.New("missing")
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Status summary:\n- Provider: (not set)\n- API provider type: Anthropic first-party\n- Model: (not set)\n- Project path: (not set)\n- Approval mode: (not set)\n- Session storage: not configured\n- Settings sources: none\n- Account auth: missing auth credential; interactive account status is not available\n- API key source: not configured\n- Auth token source: not configured\n- API base URL: default\n- API base URL source: default\n- Bash sandbox: not available in Claude Code Go yet\n- MCP servers: no MCP tools registered\n- Memory files: project path not configured\n- Installation health: ripgrep missing from PATH\n- API connectivity check: skipped (missing auth credential)\n- Tool status checks: no tools registered\n- Settings status UI: not available in Claude Code Go yet"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestStatusCommandExecuteWithLargeMemoryFile verifies /status reports oversized workspace memory files.
func TestStatusCommandExecuteWithLargeMemoryFile(t *testing.T) {
	statFn := func(path string) (os.FileInfo, error) {
		switch path {
		case "/repo/project/CLAUDE.md":
			return statusStubRegularFileInfo{}, nil
		default:
			return nil, errors.New("missing")
		}
	}

	result, err := StatusCommand{
		Config: coreconfig.Config{
			Provider:    "anthropic",
			ProjectPath: "/repo/project",
			APIKey:      "test-key",
		},
		Stat: statFn,
		ReadFile: func(path string) ([]byte, error) {
			if path != "/repo/project/CLAUDE.md" {
				return nil, errors.New("missing")
			}
			return []byte(strings.Repeat("a", maxMemoryDiagnosticBytes+1)), nil
		},
		LookPath: func(string) (string, error) {
			return "/opt/homebrew/bin/rg", nil
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(result.Output, "- Memory files: large CLAUDE.md detected: /repo/project/CLAUDE.md (40001 bytes) > 40000 bytes") {
		t.Fatalf("Execute() output = %q, want large CLAUDE.md diagnostic", result.Output)
	}
	if !strings.Contains(result.Output, "- Installation health: ripgrep available at /opt/homebrew/bin/rg") {
		t.Fatalf("Execute() output = %q, want ripgrep availability diagnostic", result.Output)
	}
}
