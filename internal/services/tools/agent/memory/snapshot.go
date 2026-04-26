package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	snapshotBase = "agent-memory-snapshots"
	snapshotJSON = "snapshot.json"
	syncedJSON   = ".snapshot-synced.json"
)

// SnapshotMeta holds the metadata stored in snapshot.json.
type SnapshotMeta struct {
	UpdatedAt string `json:"updatedAt"`
}

// SyncedMeta holds the metadata stored in .snapshot-synced.json.
type SyncedMeta struct {
	SyncedFrom string `json:"syncedFrom"`
}

// SnapshotResult describes the action required after checking a snapshot.
type SnapshotResult struct {
	// Action is one of: "none", "initialize", "prompt-update".
	Action string
	// SnapshotTimestamp is the snapshot's updatedAt value when Action is not "none".
	SnapshotTimestamp string
}

// CheckAgentMemorySnapshot checks whether a project snapshot exists for the agent
// and whether it is newer than the local memory that was last synced.
func (p *Paths) CheckAgentMemorySnapshot(agentType string, scope AgentMemoryScope) (SnapshotResult, error) {
	snapshotMeta, err := p.readSnapshotMeta(agentType)
	if err != nil || snapshotMeta == nil {
		return SnapshotResult{Action: "none"}, nil
	}

	localMemDir := p.GetAgentMemoryDir(agentType, scope)
	hasLocalMemory := false
	if entries, err := os.ReadDir(localMemDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				hasLocalMemory = true
				break
			}
		}
	}

	if !hasLocalMemory {
		return SnapshotResult{
			Action:            "initialize",
			SnapshotTimestamp: snapshotMeta.UpdatedAt,
		}, nil
	}

	syncedMeta, err := p.readSyncedMeta(agentType, scope)
	if err != nil || syncedMeta == nil {
		return SnapshotResult{
			Action:            "prompt-update",
			SnapshotTimestamp: snapshotMeta.UpdatedAt,
		}, nil
	}

	snapshotTime, err1 := time.Parse(time.RFC3339, snapshotMeta.UpdatedAt)
	syncedTime, err2 := time.Parse(time.RFC3339, syncedMeta.SyncedFrom)
	if err1 != nil || err2 != nil || snapshotTime.After(syncedTime) {
		return SnapshotResult{
			Action:            "prompt-update",
			SnapshotTimestamp: snapshotMeta.UpdatedAt,
		}, nil
	}

	return SnapshotResult{Action: "none"}, nil
}

// InitializeFromSnapshot copies snapshot files into local agent memory for the first time.
func (p *Paths) InitializeFromSnapshot(agentType string, scope AgentMemoryScope, snapshotTimestamp string) error {
	logger.DebugCF("agent.memory.snapshot", "initializing agent memory from project snapshot", map[string]any{
		"agent_type": agentType,
		"scope":      scope,
	})
	if err := p.copySnapshotToLocal(agentType, scope); err != nil {
		logger.WarnCF("agent.memory.snapshot", "failed to copy snapshot to local", map[string]any{
			"agent_type": agentType,
			"error":      err.Error(),
		})
		return err
	}
	return p.saveSyncedMeta(agentType, scope, snapshotTimestamp)
}

// ReplaceFromSnapshot replaces local agent memory files with the snapshot contents.
func (p *Paths) ReplaceFromSnapshot(agentType string, scope AgentMemoryScope, snapshotTimestamp string) error {
	logger.DebugCF("agent.memory.snapshot", "replacing agent memory with project snapshot", map[string]any{
		"agent_type": agentType,
		"scope":      scope,
	})
	localMemDir := p.GetAgentMemoryDir(agentType, scope)
	if entries, err := os.ReadDir(localMemDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				_ = os.Remove(filepath.Join(localMemDir, e.Name()))
			}
		}
	}
	if err := p.copySnapshotToLocal(agentType, scope); err != nil {
		logger.WarnCF("agent.memory.snapshot", "failed to copy snapshot to local", map[string]any{
			"agent_type": agentType,
			"error":      err.Error(),
		})
		return err
	}
	return p.saveSyncedMeta(agentType, scope, snapshotTimestamp)
}

// MarkSnapshotSynced records that the current snapshot has been synced without
// modifying local memory files.
func (p *Paths) MarkSnapshotSynced(agentType string, scope AgentMemoryScope, snapshotTimestamp string) error {
	return p.saveSyncedMeta(agentType, scope, snapshotTimestamp)
}

// snapshotDir returns the snapshot directory for an agent.
func (p *Paths) snapshotDir(agentType string) string {
	return filepath.Join(p.cwd(), ".claude", snapshotBase, sanitizeAgentTypeForPath(agentType))
}

// snapshotJSONPath returns the path to snapshot.json.
func (p *Paths) snapshotJSONPath(agentType string) string {
	return filepath.Join(p.snapshotDir(agentType), snapshotJSON)
}

// syncedJSONPath returns the path to .snapshot-synced.json.
func (p *Paths) syncedJSONPath(agentType string, scope AgentMemoryScope) string {
	return filepath.Join(p.GetAgentMemoryDir(agentType, scope), syncedJSON)
}

// readSnapshotMeta reads and parses snapshot.json.
func (p *Paths) readSnapshotMeta(agentType string) (*SnapshotMeta, error) {
	data, err := os.ReadFile(p.snapshotJSONPath(agentType))
	if err != nil {
		return nil, err
	}
	var meta SnapshotMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	if meta.UpdatedAt == "" {
		return nil, nil
	}
	return &meta, nil
}

// readSyncedMeta reads and parses .snapshot-synced.json.
func (p *Paths) readSyncedMeta(agentType string, scope AgentMemoryScope) (*SyncedMeta, error) {
	data, err := os.ReadFile(p.syncedJSONPath(agentType, scope))
	if err != nil {
		return nil, err
	}
	var meta SyncedMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	if meta.SyncedFrom == "" {
		return nil, nil
	}
	return &meta, nil
}

// saveSyncedMeta writes .snapshot-synced.json.
func (p *Paths) saveSyncedMeta(agentType string, scope AgentMemoryScope, snapshotTimestamp string) error {
	syncedPath := p.syncedJSONPath(agentType, scope)
	localMemDir := p.GetAgentMemoryDir(agentType, scope)
	if err := os.MkdirAll(localMemDir, 0755); err != nil {
		return err
	}
	meta := SyncedMeta{SyncedFrom: snapshotTimestamp}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(syncedPath, data, 0644)
}

// copySnapshotToLocal copies all files (except snapshot.json) from the snapshot
// directory into the local agent memory directory.
func (p *Paths) copySnapshotToLocal(agentType string, scope AgentMemoryScope) error {
	snapshotDir := p.snapshotDir(agentType)
	localMemDir := p.GetAgentMemoryDir(agentType, scope)

	if err := os.MkdirAll(localMemDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.IsDir() || e.Name() == snapshotJSON {
			continue
		}
		srcPath := filepath.Join(snapshotDir, e.Name())
		dstPath := filepath.Join(localMemDir, e.Name())
		data, err := os.ReadFile(srcPath)
		if err != nil {
			logger.DebugCF("agent.memory.snapshot", "failed to read snapshot file", map[string]any{
				"file":  e.Name(),
				"error": err.Error(),
			})
			continue
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			logger.DebugCF("agent.memory.snapshot", "failed to write snapshot file to local", map[string]any{
				"file":  e.Name(),
				"error": err.Error(),
			})
			continue
		}
	}
	return nil
}
