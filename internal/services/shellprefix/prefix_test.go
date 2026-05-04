package shellprefix

import (
	"context"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/services/haiku"
)

// mockQuerier implements haiku.Querier for tests.
type mockQuerier struct {
	result *haiku.QueryResult
	err    error
	calls  []haiku.QueryParams
}

func (m *mockQuerier) Query(ctx context.Context, params haiku.QueryParams) (*haiku.QueryResult, error) {
	m.calls = append(m.calls, params)
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestExtract_Success(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "python3"}}
	svc := NewService(mq)

	result, err := svc.Extract(context.Background(), "python3 script.py", "some policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "python3" {
		t.Errorf("result = %q, want %q", result, "python3")
	}
	if len(mq.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(mq.calls))
	}
	if mq.calls[0].QuerySource != querySource {
		t.Errorf("query_source = %q, want %q", mq.calls[0].QuerySource, querySource)
	}
}

func TestExtract_FlagDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "0")
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Extract(context.Background(), "python3 script.py", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
	if len(mq.calls) != 0 {
		t.Error("expected no querier calls")
	}
}

func TestExtract_EmptyCommand(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Extract(context.Background(), "   ", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
	if len(mq.calls) != 0 {
		t.Error("expected no querier calls")
	}
}

func TestExtract_NilService(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	var svc *Service

	result, err := svc.Extract(context.Background(), "python3 script.py", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestExtract_QuerierError(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{err: errors.New("haiku overloaded")}
	svc := NewService(mq)

	result, err := svc.Extract(context.Background(), "python3 script.py", "policy")
	if err != ErrHaikuCallFailed {
		t.Fatalf("expected ErrHaikuCallFailed, got %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestExtract_QuerierReturnsNilResult(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{result: nil}
	svc := NewService(mq)

	result, err := svc.Extract(context.Background(), "python3 script.py", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestExtract_QuerierReturnsEmptyText(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: ""}}
	svc := NewService(mq)

	result, err := svc.Extract(context.Background(), "python3 script.py", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestExtract_CommandInjectionDetected(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "command_injection_detected"}}
	svc := NewService(mq)

	result, err := svc.Extract(context.Background(), "rm -rf /", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestExtract_NonePrefix(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "none"}}
	svc := NewService(mq)

	result, err := svc.Extract(context.Background(), "echo hello", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestExtract_GitRejected(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "git"}}
	svc := NewService(mq)

	result, err := svc.Extract(context.Background(), "git push origin main", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestExtract_DangerousShellPrefix(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "bash"}}
	svc := NewService(mq)

	result, err := svc.Extract(context.Background(), "bash -c 'echo hi'", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestExtract_DangerousShellPrefixCaseInsensitive(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "BASH"}}
	svc := NewService(mq)

	result, err := svc.Extract(context.Background(), "BASH script.sh", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty for case-insensitive check", result)
	}
}

func TestExtract_PrefixMismatch(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "npm"}}
	svc := NewService(mq)

	// "npm" is not a prefix of "python3 script.py"
	result, err := svc.Extract(context.Background(), "python3 script.py", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty when prefix does not match command", result)
	}
}

func TestExtract_PackageLevel(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SHELL_PREFIX", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "npm"}}
	setCurrentService(NewService(mq))
	defer setCurrentService(nil)

	result, err := Generate(context.Background(), "npm install", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "npm" {
		t.Errorf("result = %q, want %q", result, "npm")
	}
}

func TestExtract_PackageLevelUninitialized(t *testing.T) {
	setCurrentService(nil)
	result, err := Generate(context.Background(), "npm install", "policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestDangerousSet_Has(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"sh", true},
		{"bash", true},
		{"zsh", true},
		{"fish", true},
		{"csh", true},
		{"tcsh", true},
		{"ksh", true},
		{"dash", true},
		{"cmd", true},
		{"cmd.exe", true},
		{"powershell", true},
		{"powershell.exe", true},
		{"pwsh", true},
		{"pwsh.exe", true},
		{"bash.exe", true},
		{"BASH", true},
		{"Git", false},
		{"python3", false},
		{"node", false},
		{"npm", false},
	}
	for _, c := range cases {
		got := dangerousShellPrefixes.has(c.input)
		if got != c.want {
			t.Errorf("dangerousSet.has(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}
