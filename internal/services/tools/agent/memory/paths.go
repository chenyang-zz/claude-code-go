package memory

import (
	"os"
	"path/filepath"
	"strings"
)

// AgentMemoryScope defines the persistence scope for agent memory.
type AgentMemoryScope string

const (
	// ScopeUser persists memory under the user's home directory (~/.claude/agent-memory/).
	ScopeUser AgentMemoryScope = "user"
	// ScopeProject persists memory under the current project's .claude directory.
	ScopeProject AgentMemoryScope = "project"
	// ScopeLocal persists memory under the current project's local directory (not version-controlled).
	ScopeLocal AgentMemoryScope = "local"
)

// Paths provides path resolution for agent memory directories.
// Callers should set CWD and MemoryBaseDir; when empty, sensible defaults are used.
type Paths struct {
	// CWD is the working directory used for project and local scope resolution.
	// Defaults to os.Getwd() when empty.
	CWD string
	// MemoryBaseDir is the base directory for user-scope memory.
	// Defaults to ~/.claude when empty.
	MemoryBaseDir string
}

// cwd returns the effective working directory.
func (p *Paths) cwd() string {
	if p.CWD != "" {
		return p.CWD
	}
	cwd, _ := os.Getwd()
	return cwd
}

// memoryBaseDir returns the effective memory base directory.
func (p *Paths) memoryBaseDir() string {
	if p.MemoryBaseDir != "" {
		return p.MemoryBaseDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// sanitizeAgentTypeForPath replaces colons (used in plugin-namespaced agent types)
// with dashes to produce a valid directory name.
func sanitizeAgentTypeForPath(agentType string) string {
	return strings.ReplaceAll(agentType, ":", "-")
}

// GetAgentMemoryDir returns the agent memory directory for the given agent type and scope.
//
//	user scope:    <memoryBaseDir>/agent-memory/<agentType>/
//	project scope: <cwd>/.claude/agent-memory/<agentType>/
//	local scope:   <cwd>/.claude/agent-memory-local/<agentType>/
func (p *Paths) GetAgentMemoryDir(agentType string, scope AgentMemoryScope) string {
	dirName := sanitizeAgentTypeForPath(agentType)
	switch scope {
	case ScopeProject:
		return filepath.Join(p.cwd(), ".claude", "agent-memory", dirName) + string(filepath.Separator)
	case ScopeLocal:
		return filepath.Join(p.cwd(), ".claude", "agent-memory-local", dirName) + string(filepath.Separator)
	case ScopeUser:
		return filepath.Join(p.memoryBaseDir(), "agent-memory", dirName) + string(filepath.Separator)
	default:
		return ""
	}
}

// GetAgentMemoryEntrypoint returns the path to the agent's MEMORY.md file.
func (p *Paths) GetAgentMemoryEntrypoint(agentType string, scope AgentMemoryScope) string {
	return filepath.Join(p.GetAgentMemoryDir(agentType, scope), "MEMORY.md")
}

// IsAgentMemoryPath reports whether absolutePath lies within any agent memory directory.
// It normalizes the path to prevent traversal bypasses via ".." segments.
func (p *Paths) IsAgentMemoryPath(absolutePath string) bool {
	normalized := filepath.Clean(absolutePath)

	// User scope.
	userPrefix := filepath.Join(p.memoryBaseDir(), "agent-memory") + string(filepath.Separator)
	if strings.HasPrefix(normalized, userPrefix) {
		return true
	}

	// Project scope.
	projectPrefix := filepath.Join(p.cwd(), ".claude", "agent-memory") + string(filepath.Separator)
	if strings.HasPrefix(normalized, projectPrefix) {
		return true
	}

	// Local scope.
	localPrefix := filepath.Join(p.cwd(), ".claude", "agent-memory-local") + string(filepath.Separator)
	if strings.HasPrefix(normalized, localPrefix) {
		return true
	}

	return false
}

// EnsureAgentMemoryDir creates the memory directory (and any parent directories)
// if they do not already exist. It is idempotent.
func EnsureAgentMemoryDir(memoryDir string) error {
	return os.MkdirAll(memoryDir, 0755)
}
