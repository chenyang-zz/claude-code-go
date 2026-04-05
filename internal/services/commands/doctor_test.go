package commands

import (
	"context"
	"errors"
	"os"
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
			Provider:      "anthropic",
			Model:         "claude-sonnet-4-5",
			ProjectPath:   "/repo",
			ApprovalMode:  "default",
			SessionDBPath: "/tmp/claude.db",
		},
		Stat: statFn,
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Doctor summary:\n- Provider: anthropic\n- Model: claude-sonnet-4-5\n- API key: missing\n- API base URL: default\n- Project path: /repo\n- Approval mode: default\n- Session DB: /tmp/claude.db (not created yet; parent directory exists)"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
