package wiring

import (
	"testing"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// TestBaseWorkspaceToolsIncludesBash verifies the migrated foreground Bash tool joins the default runtime toolset.
func TestBaseWorkspaceToolsIncludesBash(t *testing.T) {
	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	modules, err := NewBaseWorkspaceModules(platformfs.NewLocalFS(), policy, coreconfig.PermissionConfig{
		Allow: []string{"Bash(*)"},
	}, nil)
	if err != nil {
		t.Fatalf("NewBaseWorkspaceModules() error = %v", err)
	}

	if _, ok := modules.Tools.Get("Bash"); !ok {
		t.Fatal("NewBaseWorkspaceModules() registry missing Bash tool")
	}
	if _, ok := modules.Tools.Get("TaskStop"); !ok {
		t.Fatal("NewBaseWorkspaceModules() registry missing TaskStop tool")
	}
}
