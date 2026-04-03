package wiring

import (
	"testing"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// TestNewBaseWorkspaceModulesRegistersCanonicalToolList verifies the base workspace tools are wired in one stable, enumerable order.
func TestNewBaseWorkspaceModulesRegistersCanonicalToolList(t *testing.T) {
	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	modules, err := NewBaseWorkspaceModules(platformfs.NewLocalFS(), policy)
	if err != nil {
		t.Fatalf("NewBaseWorkspaceModules() error = %v", err)
	}

	registered := modules.Tools.List()
	if len(registered) != 5 {
		t.Fatalf("List() len = %d, want 5", len(registered))
	}

	wantNames := []string{"Glob", "Grep", "Read", "Write", "Edit"}
	for index, tool := range registered {
		if tool.Name() != wantNames[index] {
			t.Fatalf("List()[%d] name = %q, want %q", index, tool.Name(), wantNames[index])
		}
	}
}
