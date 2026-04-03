package permission

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFilesystemPolicy(t *testing.T) {
	t.Parallel()

	policy, err := NewFilesystemPolicy(RuleSet{
		Read: []Rule{
			{
				Source:   RuleSourceSession,
				Decision: DecisionAllow,
				Pattern:  "src/**",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	if policy == nil {
		t.Fatal("NewFilesystemPolicy() returned nil policy")
	}
}

func TestFilesystemPolicyEvaluateFilesystemRead(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(string(filepath.Separator), "workspace")

	policy, err := NewFilesystemPolicy(RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tests := []struct {
		name         string
		req          FilesystemRequest
		wantDecision Decision
		wantMessage  string
	}{
		{
			name: "allow relative path inside working dir",
			req: FilesystemRequest{
				ToolName:   "file_read",
				Path:       "README.md",
				WorkingDir: workspace,
				Access:     AccessRead,
			},
			wantDecision: DecisionAllow,
		},
		{
			name: "allow absolute path inside working dir",
			req: FilesystemRequest{
				ToolName:   "glob",
				Path:       filepath.Join(workspace, "src", "main.go"),
				WorkingDir: workspace,
				Access:     AccessRead,
			},
			wantDecision: DecisionAllow,
		},
		{
			name: "ask absolute path outside working dir",
			req: FilesystemRequest{
				ToolName:   "file_read",
				Path:       filepath.Join(string(filepath.Separator), "etc", "hosts"),
				WorkingDir: workspace,
				Access:     AccessRead,
			},
			wantDecision: DecisionAsk,
			wantMessage:  "Claude requested permissions to read from",
		},
		{
			name: "deny invalid request",
			req: FilesystemRequest{
				Path:       "README.md",
				WorkingDir: workspace,
				Access:     AccessRead,
			},
			wantDecision: DecisionDeny,
			wantMessage:  "permission: invalid filesystem request",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := policy.EvaluateFilesystem(context.Background(), tt.req)
			if got.Decision != tt.wantDecision {
				t.Fatalf("EvaluateFilesystem() decision = %q, want %q", got.Decision, tt.wantDecision)
			}
			if tt.wantMessage != "" && !strings.Contains(got.Message, tt.wantMessage) {
				t.Fatalf("EvaluateFilesystem() message = %q, want substring %q", got.Message, tt.wantMessage)
			}
		})
	}
}

func TestFilesystemPolicyEvaluateFilesystemWriteDefaultsToAsk(t *testing.T) {
	t.Parallel()

	policy, err := NewFilesystemPolicy(RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	got := policy.EvaluateFilesystem(context.Background(), FilesystemRequest{
		ToolName:   "file_write",
		Path:       "output.txt",
		WorkingDir: filepath.Join(string(filepath.Separator), "workspace"),
		Access:     AccessWrite,
	})

	if got.Decision != DecisionAsk {
		t.Fatalf("EvaluateFilesystem() decision = %q, want %q", got.Decision, DecisionAsk)
	}
}

func TestNormalizeFilesystemRequestPath(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(string(filepath.Separator), "workspace")
	gotPath, gotWorkingDir, err := normalizeFilesystemRequestPath("src/main.go", workspace)
	if err != nil {
		t.Fatalf("normalizeFilesystemRequestPath() error = %v", err)
	}

	wantPath := filepath.Join(workspace, "src", "main.go")
	if gotPath != wantPath {
		t.Fatalf("normalizeFilesystemRequestPath() path = %q, want %q", gotPath, wantPath)
	}
	if gotWorkingDir != workspace {
		t.Fatalf("normalizeFilesystemRequestPath() working dir = %q, want %q", gotWorkingDir, workspace)
	}
}
