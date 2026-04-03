package wiring

import (
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	fileedit "github.com/sheepzhao/claude-code-go/internal/services/tools/file_edit"
	fileread "github.com/sheepzhao/claude-code-go/internal/services/tools/file_read"
	filewrite "github.com/sheepzhao/claude-code-go/internal/services/tools/file_write"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/glob"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/grep"
)

// Modules aggregates host-level runtime dependencies assembled during startup.
type Modules struct {
	// Tools is the registry exposed to executors for tool lookup and dispatch.
	Tools tool.Registry
}

// NewModules wires the provided tools into the default in-memory registry.
func NewModules(tools ...tool.Tool) (Modules, error) {
	registry := tool.NewMemoryRegistry()
	for _, item := range tools {
		if err := registry.Register(item); err != nil {
			return Modules{}, err
		}
	}

	return Modules{
		Tools: registry,
	}, nil
}

// NewBaseWorkspaceModules wires the base workspace exploration and editing tools into one registry.
func NewBaseWorkspaceModules(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy) (Modules, error) {
	return NewModules(BaseWorkspaceTools(fs, policy)...)
}

// BaseWorkspaceTools returns the canonical registration list for the base workspace toolset.
func BaseWorkspaceTools(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy) []tool.Tool {
	return []tool.Tool{
		glob.NewTool(fs, policy),
		grep.NewTool(fs, policy),
		fileread.NewTool(fs, policy),
		filewrite.NewTool(fs, policy),
		fileedit.NewTool(fs, policy),
	}
}
