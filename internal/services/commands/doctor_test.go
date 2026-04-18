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
)

type stubFileInfo struct{}

func (stubFileInfo) Name() string       { return "" }
func (stubFileInfo) Size() int64        { return 0 }
func (stubFileInfo) Mode() os.FileMode  { return 0 }
func (stubFileInfo) ModTime() time.Time { return time.Time{} }
func (stubFileInfo) IsDir() bool        { return true }
func (stubFileInfo) Sys() any           { return nil }

// TestDoctorCommandExecuteRendersLocalDiagnostics verifies /doctor reports a stable local diagnosis without remote probes.
func TestDoctorCommandExecuteRendersLocalDiagnostics(t *testing.T) {
	statFn := func(path string) (os.FileInfo, error) {
		switch path {
		case "/tmp":
			return stubFileInfo{}, nil
		default:
			return nil, errors.New("missing")
		}
	}

	result, err := DoctorCommand{
		Config: coreconfig.Config{
			Provider:              "anthropic",
			Model:                 "claude-sonnet-4-5",
			ProjectPath:           "/repo",
			ApprovalMode:          "default",
			SessionDBPath:         "/tmp/claude.db",
			ProxyURL:              "http://proxy.internal:8080",
			AdditionalCACertsPath: "/etc/ssl/custom.pem",
			MTLSClientCertPath:    "/etc/ssl/client.pem",
			MTLSClientKeyPath:     "/etc/ssl/client-key.pem",
		},
		Stat: statFn,
		LookPath: func(string) (string, error) {
			return "", errors.New("missing")
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Doctor summary:\n- Provider: anthropic\n- Model: claude-sonnet-4-5\n- API key: missing\n- API base URL: default\n- Project path: /repo\n- Approval mode: default\n- Session DB: /tmp/claude.db (not created yet; parent directory exists)\n- Proxy: http://proxy.internal:8080\n- Additional CA cert(s): /etc/ssl/custom.pem\n- mTLS client cert: /etc/ssl/client.pem\n- mTLS client key: /etc/ssl/client-key.pem\n- Bash sandbox: not available in Claude Code Go yet\n- IDE: not detected\n- MCP servers: no MCP tools registered\n- Memory files: no CLAUDE.md files detected\n- Installation health: ripgrep missing from PATH"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestDoctorCommandExecuteWithTerminalIDE verifies /doctor shares the terminal IDE fallback.
func TestDoctorCommandExecuteWithTerminalIDE(t *testing.T) {
	result, err := DoctorCommand{
		Config: coreconfig.Config{
			Provider:    "anthropic",
			ProjectPath: "/repo",
		},
		LookupEnv: func(key string) (string, bool) {
			switch key {
			case "JETBRAINS_IDE":
				return "GoLand", true
			default:
				return "", false
			}
		},
		LookPath: func(string) (string, error) {
			return "", errors.New("missing")
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(result.Output, "- IDE: terminal session appears to be running inside GoLand") {
		t.Fatalf("Execute() output = %q, want terminal IDE diagnostic", result.Output)
	}
}
