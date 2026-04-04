package permission

import (
	"context"
	"errors"
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

	policy, err := NewFilesystemPolicy(RuleSet{
		Read: []Rule{
			{
				Source:   RuleSourceSession,
				Decision: DecisionDeny,
				Pattern:  "secrets/**",
			},
			{
				Source:   RuleSourceProjectSettings,
				Decision: DecisionAsk,
				Pattern:  "gated/**",
			},
			{
				Source:   RuleSourceSession,
				Decision: DecisionAllow,
				BaseDir:  filepath.Join(workspace, "vendor"),
				Pattern:  "**/*.md",
			},
		},
	})
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
			name: "deny relative path by rule inside working dir",
			req: FilesystemRequest{
				ToolName:   "file_read",
				Path:       "secrets/token.txt",
				WorkingDir: workspace,
				Access:     AccessRead,
			},
			wantDecision: DecisionDeny,
			wantMessage:  "Permission to read secrets/token.txt has been denied.",
		},
		{
			name: "ask relative path by rule inside working dir",
			req: FilesystemRequest{
				ToolName:   "file_read",
				Path:       "gated/spec.md",
				WorkingDir: workspace,
				Access:     AccessRead,
			},
			wantDecision: DecisionAsk,
			wantMessage:  "Claude requested permissions to read from gated/spec.md",
		},
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
			name: "allow outside working dir by explicit rule",
			req: FilesystemRequest{
				ToolName:   "file_read",
				Path:       filepath.Join(workspace, "vendor", "docs", "guide.md"),
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
			if got.Decision != DecisionAllow && got.ToError(tt.req) == nil {
				t.Fatal("EvaluateFilesystem().ToError() = nil, want permission error")
			}
		})
	}
}

func TestFilesystemPolicyEvaluateFilesystemWriteDefaultsToAsk(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(string(filepath.Separator), "workspace")

	policy, err := NewFilesystemPolicy(RuleSet{
		Write: []Rule{
			{
				Source:   RuleSourceSession,
				Decision: DecisionDeny,
				Pattern:  "protected/**",
			},
			{
				Source:   RuleSourceSession,
				Decision: DecisionAsk,
				Pattern:  "review/**",
			},
			{
				Source:   RuleSourceProjectSettings,
				Decision: DecisionAllow,
				Pattern:  "scratch/**",
			},
		},
	})
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
			name: "deny write by rule",
			req: FilesystemRequest{
				ToolName:   "file_write",
				Path:       "protected/config.yaml",
				WorkingDir: workspace,
				Access:     AccessWrite,
			},
			wantDecision: DecisionDeny,
			wantMessage:  "Permission to write protected/config.yaml has been denied.",
		},
		{
			name: "ask write by rule",
			req: FilesystemRequest{
				ToolName:   "file_write",
				Path:       "review/output.txt",
				WorkingDir: workspace,
				Access:     AccessWrite,
			},
			wantDecision: DecisionAsk,
			wantMessage:  "Claude requested permissions to write to review/output.txt",
		},
		{
			name: "allow write by rule",
			req: FilesystemRequest{
				ToolName:   "file_write",
				Path:       "scratch/output.txt",
				WorkingDir: workspace,
				Access:     AccessWrite,
			},
			wantDecision: DecisionAllow,
		},
		{
			name: "default write ask without rule",
			req: FilesystemRequest{
				ToolName:   "file_write",
				Path:       "output.txt",
				WorkingDir: workspace,
				Access:     AccessWrite,
			},
			wantDecision: DecisionAsk,
			wantMessage:  "Claude requested permissions to write to output.txt",
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

func TestFilesystemPolicyCheckReadPermissionForTool(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(string(filepath.Separator), "workspace")

	policy, err := NewFilesystemPolicy(RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	got := policy.CheckReadPermissionForTool(context.Background(), "file_read", "README.md", workspace)
	if got.Decision != DecisionAllow {
		t.Fatalf("CheckReadPermissionForTool() decision = %q, want %q", got.Decision, DecisionAllow)
	}
}

func TestFilesystemPolicyCheckReadPermissionForGlob(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(string(filepath.Separator), "workspace")
	externalRoot := filepath.Join(string(filepath.Separator), "external", "vendor")

	policy, err := NewFilesystemPolicy(RuleSet{
		Read: []Rule{
			{
				Source:   RuleSourceSession,
				Decision: DecisionAllow,
				BaseDir:  externalRoot,
				Pattern:  "**/*.md",
			},
			{
				Source:   RuleSourceSession,
				Decision: DecisionDeny,
				BaseDir:  externalRoot,
				Pattern:  "private/**",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	allowed := policy.CheckReadPermissionForGlob(
		context.Background(),
		"glob",
		externalRoot,
		workspace,
		"**/*.md",
	)
	if allowed.Decision != DecisionAllow {
		t.Fatalf("CheckReadPermissionForGlob() allow decision = %q, want %q", allowed.Decision, DecisionAllow)
	}

	denied := policy.CheckReadPermissionForGlob(
		context.Background(),
		"glob",
		externalRoot,
		workspace,
		"private/*.txt",
	)
	if denied.Decision != DecisionDeny {
		t.Fatalf("CheckReadPermissionForGlob() deny decision = %q, want %q", denied.Decision, DecisionDeny)
	}

	asked := policy.CheckReadPermissionForGlob(
		context.Background(),
		"glob",
		externalRoot,
		workspace,
		"**/*.txt",
	)
	if asked.Decision != DecisionAsk {
		t.Fatalf("CheckReadPermissionForGlob() ask decision = %q, want %q", asked.Decision, DecisionAsk)
	}
}

func TestFilesystemPolicyCheckWritePermissionForTool(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(string(filepath.Separator), "workspace")

	policy, err := NewFilesystemPolicy(RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	got := policy.CheckWritePermissionForTool(context.Background(), "file_write", "output.txt", workspace)
	if got.Decision != DecisionAsk {
		t.Fatalf("CheckWritePermissionForTool() decision = %q, want %q", got.Decision, DecisionAsk)
	}
	if !strings.Contains(got.Message, "Claude requested permissions to write to") {
		t.Fatalf("CheckWritePermissionForTool() message = %q, want write approval prompt", got.Message)
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

func TestMatchPermissionPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		{name: "exact file", path: "README.md", pattern: "README.md", want: true},
		{name: "single segment wildcard", path: "src/main.go", pattern: "src/*.go", want: true},
		{name: "double star nested", path: "vendor/docs/guide.md", pattern: "**/*.md", want: true},
		{name: "directory wildcard suffix", path: "secrets/token.txt", pattern: "secrets/**", want: true},
		{name: "outside pattern", path: "src/main.go", pattern: "docs/**", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := matchPermissionPattern(tt.path, tt.pattern); got != tt.want {
				t.Fatalf("matchPermissionPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestEvaluationToError(t *testing.T) {
	t.Parallel()

	req := FilesystemRequest{
		ToolName:   "file_read",
		Path:       "secrets/token.txt",
		WorkingDir: filepath.Join(string(filepath.Separator), "workspace"),
		Access:     AccessRead,
	}
	rule := &Rule{
		Source:   RuleSourceSession,
		Decision: DecisionDeny,
		Pattern:  "secrets/**",
	}

	err := (Evaluation{
		Decision: DecisionDeny,
		Rule:     rule,
		Message:  "Permission to read secrets/token.txt has been denied.",
	}).ToError(req)
	if err == nil {
		t.Fatal("Evaluation.ToError() = nil, want permission error")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatal("Evaluation.ToError() error is not recognized as ErrUnauthorized")
	}

	var permissionErr *PermissionError
	if !errors.As(err, &permissionErr) {
		t.Fatal("Evaluation.ToError() error is not a *PermissionError")
	}
	if permissionErr.Decision != DecisionDeny {
		t.Fatalf("PermissionError.Decision = %q, want %q", permissionErr.Decision, DecisionDeny)
	}
	if permissionErr.Rule != rule {
		t.Fatal("PermissionError.Rule does not retain matched rule")
	}
	if got := (Evaluation{Decision: DecisionAllow}).ToError(req); got != nil {
		t.Fatalf("Evaluation.ToError() for allow = %v, want nil", got)
	}
}
