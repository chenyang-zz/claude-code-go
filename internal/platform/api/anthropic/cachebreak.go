package anthropic

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// maxTrackedSources caps the number of tracked sources to prevent unbounded memory growth.
// Each entry stores a diffable content string (serialized system prompt + tool schemas).
// Without a cap, spawning many subagents causes the map to grow indefinitely.
const maxTrackedSources = 10

// minCacheMissTokens is the minimum absolute token drop required to trigger a cache break warning.
const minCacheMissTokens = 2000

// trackedSourcePrefixes lists the source prefixes eligible for cache break tracking.
var trackedSourcePrefixes = []string{
	"repl_main_thread",
	"sdk",
	"agent:custom",
	"agent:default",
	"agent:builtin",
}

// pendingChanges captures what changed between the previous call and the current one.
type pendingChanges struct {
	systemPromptChanged        bool
	toolSchemasChanged         bool
	modelChanged               bool
	fastModeChanged            bool
	cacheControlChanged        bool
	globalCacheStrategyChanged bool
	betasChanged               bool
	autoModeChanged            bool
	overageChanged             bool
	cachedMCChanged            bool
	effortChanged              bool
	extraBodyChanged           bool
	addedToolCount             int
	removedToolCount           int
	systemCharDelta            int
	addedTools                 []string
	removedTools               []string
	changedToolSchemas         []string
	previousModel              string
	newModel                   string
	prevGlobalCacheStrategy    string
	newGlobalCacheStrategy     string
	addedBetas                 []string
	removedBetas               []string
	prevEffortValue            string
	newEffortValue             string
	buildPrevDiffableContent   func() string
}

// previousState stores the tracked state for one source between API calls.
type previousState struct {
	systemHash            uint64
	toolsHash             uint64
	cacheControlHash      uint64
	toolNames             []string
	perToolHashes         map[string]uint64
	systemCharCount       int
	model                 string
	fastMode              bool
	globalCacheStrategy   string
	betas                 []string
	autoModeActive        bool
	isUsingOverage        bool
	cachedMCEnabled       bool
	effortValue           string
	extraBodyHash         uint64
	callCount             int
	pendingChanges        *pendingChanges
	prevCacheReadTokens   *int
	cacheDeletionsPending bool
	buildDiffableContent  func() string
}

// PromptStateSnapshot captures everything that could affect the server-side
// cache key that we can observe from the client. Zero values compare as stable.
type PromptStateSnapshot struct {
	System              string
	Tools               []model.ToolDefinition
	Source              string
	Model               string
	AgentID             string
	FastMode            bool
	GlobalCacheStrategy string
	Betas               []string
	AutoModeActive      bool
	IsUsingOverage      bool
	CachedMCEnabled     bool
	EffortValue         string
	ExtraBodyParams     any
	EnablePromptCaching bool
}

// CacheBreakDetector tracks prompt state across API calls and detects unexpected
// Anthropic prompt cache breaks by comparing cache read token counts.
type CacheBreakDetector struct {
	mu     sync.RWMutex
	states map[string]*previousState
}

// NewCacheBreakDetector creates a new cache break detector with an empty state map.
func NewCacheBreakDetector() *CacheBreakDetector {
	return &CacheBreakDetector{
		states: make(map[string]*previousState),
	}
}

// getTrackingKey returns the tracking key for a source, or empty string if untracked.
// Compact shares the same server-side cache as repl_main_thread.
// Subagents use their unique agentId to isolate tracking state.
func getTrackingKey(source, agentID string) string {
	if source == "compact" {
		return "repl_main_thread"
	}
	for _, prefix := range trackedSourcePrefixes {
		if strings.HasPrefix(source, prefix) {
			if strings.TrimSpace(agentID) != "" {
				return agentID
			}
			return source
		}
	}
	return ""
}

// isExcludedModel returns true for models with different caching behavior.
func isExcludedModel(model string) bool {
	return strings.Contains(model, "haiku")
}

// djb2HashUint64 computes the djb2 hash and returns it as a uint64.
func djb2HashUint64(s string) uint64 {
	var hash uint64 = 5381
	for i := 0; i < len(s); i++ {
		hash = ((hash << 5) + hash) + uint64(s[i])
	}
	return hash
}

// computeHash serializes data to JSON and computes its djb2 hash.
func computeHash(data any) uint64 {
	b, err := json.Marshal(data)
	if err != nil {
		return 0
	}
	return djb2HashUint64(string(b))
}

// computePerToolHashes returns a map from tool name to its schema hash.
func computePerToolHashes(tools []model.ToolDefinition) map[string]uint64 {
	hashes := make(map[string]uint64, len(tools))
	for i, tool := range tools {
		name := tool.Name
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("__idx_%d", i)
		}
		hashes[name] = computeHash(tool)
	}
	return hashes
}

// buildDiffableContent creates a human-readable snapshot of system prompt and tools.
func buildDiffableContent(system string, tools []model.ToolDefinition, modelName string) string {
	var toolDetails []string
	for _, t := range tools {
		schema, _ := json.Marshal(t.InputSchema)
		toolDetails = append(toolDetails, fmt.Sprintf("%s\n  description: %s\n  input_schema: %s",
			t.Name, t.Description, string(schema)))
	}
	sort.Strings(toolDetails)
	return fmt.Sprintf("Model: %s\n\n=== System Prompt ===\n\n%s\n\n=== Tools (%d) ===\n\n%s\n",
		modelName, system, len(tools), strings.Join(toolDetails, "\n\n"))
}

// RecordPromptState stores the current prompt state and detects what changed
// relative to the previous call. This is Phase 1 (pre-call).
func (d *CacheBreakDetector) RecordPromptState(snapshot PromptStateSnapshot) {
	key := getTrackingKey(snapshot.Source, snapshot.AgentID)
	if key == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	systemHash := djb2HashUint64(snapshot.System)
	toolsHash := computeHash(snapshot.Tools)
	cacheControlHash := computeHash(snapshot.EnablePromptCaching)

	toolNames := make([]string, 0, len(snapshot.Tools))
	for _, t := range snapshot.Tools {
		toolNames = append(toolNames, t.Name)
	}

	systemCharCount := len(snapshot.System)
	isFastMode := snapshot.FastMode
	sortedBetas := make([]string, len(snapshot.Betas))
	copy(sortedBetas, snapshot.Betas)
	sort.Strings(sortedBetas)
	effortStr := ""
	if snapshot.EffortValue != "" {
		effortStr = snapshot.EffortValue
	}
	extraBodyHash := uint64(0)
	if snapshot.ExtraBodyParams != nil {
		extraBodyHash = computeHash(snapshot.ExtraBodyParams)
	}

	prev, exists := d.states[key]
	if !exists {
		// Evict oldest entries if map is at capacity.
		for len(d.states) >= maxTrackedSources {
			var oldest string
			for k := range d.states {
				oldest = k
				break
			}
			if oldest != "" {
				delete(d.states, oldest)
			}
		}

		d.states[key] = &previousState{
			systemHash:            systemHash,
			toolsHash:             toolsHash,
			cacheControlHash:      cacheControlHash,
			toolNames:             toolNames,
			perToolHashes:         computePerToolHashes(snapshot.Tools),
			systemCharCount:       systemCharCount,
			model:                 snapshot.Model,
			fastMode:              isFastMode,
			globalCacheStrategy:   snapshot.GlobalCacheStrategy,
			betas:                 sortedBetas,
			autoModeActive:        snapshot.AutoModeActive,
			isUsingOverage:        snapshot.IsUsingOverage,
			cachedMCEnabled:       snapshot.CachedMCEnabled,
			effortValue:           effortStr,
			extraBodyHash:         extraBodyHash,
			callCount:             1,
			pendingChanges:        nil,
			prevCacheReadTokens:   nil,
			cacheDeletionsPending: false,
			buildDiffableContent: func() string {
				return buildDiffableContent(snapshot.System, snapshot.Tools, snapshot.Model)
			},
		}
		return
	}

	prev.callCount++

	systemPromptChanged := systemHash != prev.systemHash
	toolSchemasChanged := toolsHash != prev.toolsHash
	modelChanged := snapshot.Model != prev.model
	fastModeChanged := isFastMode != prev.fastMode
	cacheControlChanged := cacheControlHash != prev.cacheControlHash
	globalCacheStrategyChanged := snapshot.GlobalCacheStrategy != prev.globalCacheStrategy

	betasChanged := len(sortedBetas) != len(prev.betas)
	if !betasChanged {
		for i, b := range sortedBetas {
			if b != prev.betas[i] {
				betasChanged = true
				break
			}
		}
	}

	autoModeChanged := snapshot.AutoModeActive != prev.autoModeActive
	overageChanged := snapshot.IsUsingOverage != prev.isUsingOverage
	cachedMCChanged := snapshot.CachedMCEnabled != prev.cachedMCEnabled
	effortChanged := effortStr != prev.effortValue
	extraBodyChanged := extraBodyHash != prev.extraBodyHash

	if systemPromptChanged || toolSchemasChanged || modelChanged || fastModeChanged ||
		cacheControlChanged || globalCacheStrategyChanged || betasChanged ||
		autoModeChanged || overageChanged || cachedMCChanged || effortChanged || extraBodyChanged {

		prevToolSet := make(map[string]struct{}, len(prev.toolNames))
		for _, n := range prev.toolNames {
			prevToolSet[n] = struct{}{}
		}
		newToolSet := make(map[string]struct{}, len(toolNames))
		for _, n := range toolNames {
			newToolSet[n] = struct{}{}
		}
		prevBetaSet := make(map[string]struct{}, len(prev.betas))
		for _, b := range prev.betas {
			prevBetaSet[b] = struct{}{}
		}
		newBetaSet := make(map[string]struct{}, len(sortedBetas))
		for _, b := range sortedBetas {
			newBetaSet[b] = struct{}{}
		}

		var addedTools, removedTools []string
		for _, n := range toolNames {
			if _, ok := prevToolSet[n]; !ok {
				addedTools = append(addedTools, n)
			}
		}
		for _, n := range prev.toolNames {
			if _, ok := newToolSet[n]; !ok {
				removedTools = append(removedTools, n)
			}
		}

		var changedToolSchemas []string
		if toolSchemasChanged {
			newHashes := computePerToolHashes(snapshot.Tools)
			for _, name := range toolNames {
				if _, ok := prevToolSet[name]; !ok {
					continue
				}
				if newHashes[name] != prev.perToolHashes[name] {
					changedToolSchemas = append(changedToolSchemas, name)
				}
			}
			prev.perToolHashes = newHashes
		}

		var addedBetas, removedBetas []string
		for _, b := range sortedBetas {
			if _, ok := prevBetaSet[b]; !ok {
				addedBetas = append(addedBetas, b)
			}
		}
		for _, b := range prev.betas {
			if _, ok := newBetaSet[b]; !ok {
				removedBetas = append(removedBetas, b)
			}
		}

		prevDiffable := prev.buildDiffableContent
		prev.pendingChanges = &pendingChanges{
			systemPromptChanged:        systemPromptChanged,
			toolSchemasChanged:         toolSchemasChanged,
			modelChanged:               modelChanged,
			fastModeChanged:            fastModeChanged,
			cacheControlChanged:        cacheControlChanged,
			globalCacheStrategyChanged: globalCacheStrategyChanged,
			betasChanged:               betasChanged,
			autoModeChanged:            autoModeChanged,
			overageChanged:             overageChanged,
			cachedMCChanged:            cachedMCChanged,
			effortChanged:              effortChanged,
			extraBodyChanged:           extraBodyChanged,
			addedToolCount:             len(addedTools),
			removedToolCount:           len(removedTools),
			addedTools:                 addedTools,
			removedTools:               removedTools,
			changedToolSchemas:         changedToolSchemas,
			systemCharDelta:            systemCharCount - prev.systemCharCount,
			previousModel:              prev.model,
			newModel:                   snapshot.Model,
			prevGlobalCacheStrategy:    prev.globalCacheStrategy,
			newGlobalCacheStrategy:     snapshot.GlobalCacheStrategy,
			addedBetas:                 addedBetas,
			removedBetas:               removedBetas,
			prevEffortValue:            prev.effortValue,
			newEffortValue:             effortStr,
			buildPrevDiffableContent: func() string {
				if prevDiffable != nil {
					return prevDiffable()
				}
				return ""
			},
		}
	} else {
		prev.pendingChanges = nil
	}

	prev.systemHash = systemHash
	prev.toolsHash = toolsHash
	prev.cacheControlHash = cacheControlHash
	prev.toolNames = toolNames
	prev.systemCharCount = systemCharCount
	prev.model = snapshot.Model
	prev.fastMode = isFastMode
	prev.globalCacheStrategy = snapshot.GlobalCacheStrategy
	prev.betas = sortedBetas
	prev.autoModeActive = snapshot.AutoModeActive
	prev.isUsingOverage = snapshot.IsUsingOverage
	prev.cachedMCEnabled = snapshot.CachedMCEnabled
	prev.effortValue = effortStr
	prev.extraBodyHash = extraBodyHash
	prev.buildDiffableContent = func() string {
		return buildDiffableContent(snapshot.System, snapshot.Tools, snapshot.Model)
	}
}

// CheckResponseForCacheBreak checks the API response's cache tokens to determine
// if a cache break actually occurred. This is Phase 2 (post-call).
func (d *CacheBreakDetector) CheckResponseForCacheBreak(
	source string,
	cacheReadTokens int,
	cacheCreationTokens int,
	messages []message.Message,
	agentID string,
	requestID string,
) {
	key := getTrackingKey(source, agentID)
	if key == "" {
		return
	}

	d.mu.Lock()
	state := d.states[key]
	if state == nil {
		d.mu.Unlock()
		return
	}

	if isExcludedModel(state.model) {
		d.mu.Unlock()
		return
	}

	prevCacheRead := state.prevCacheReadTokens
	state.prevCacheReadTokens = &cacheReadTokens

	// Calculate time since last assistant message for TTL detection.
	var timeSinceLastAssistantMsg *time.Duration
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.RoleAssistant {
			delta := time.Since(time.Now()) // placeholder; we don't have timestamp in message.Message
			// Actually message.Message doesn't have Timestamp field in Go.
			// Skip TTL detection based on message timestamps.
			_ = delta
			break
		}
	}
	_ = timeSinceLastAssistantMsg

	// Skip the first call — no previous value to compare against.
	if prevCacheRead == nil {
		d.mu.Unlock()
		return
	}

	changes := state.pendingChanges

	// Cache deletions intentionally reduce the cached prefix.
	if state.cacheDeletionsPending {
		state.cacheDeletionsPending = false
		logger.DebugCF("prompt_cache", "cache deletion applied, cache read: %d → %d (expected drop)", map[string]any{
			"prev": *prevCacheRead,
			"curr": cacheReadTokens,
		})
		state.pendingChanges = nil
		d.mu.Unlock()
		return
	}

	// Detect cache break: cache read dropped >5% AND absolute drop exceeds threshold.
	tokenDrop := *prevCacheRead - cacheReadTokens
	if cacheReadTokens >= int(float64(*prevCacheRead)*0.95) || tokenDrop < minCacheMissTokens {
		state.pendingChanges = nil
		d.mu.Unlock()
		return
	}

	// Build explanation from pending changes.
	var parts []string
	if changes != nil {
		if changes.modelChanged {
			parts = append(parts, fmt.Sprintf("model changed (%s → %s)", changes.previousModel, changes.newModel))
		}
		if changes.systemPromptChanged {
			charInfo := ""
			if changes.systemCharDelta > 0 {
				charInfo = fmt.Sprintf(" (+%d chars)", changes.systemCharDelta)
			} else if changes.systemCharDelta < 0 {
				charInfo = fmt.Sprintf(" (%d chars)", changes.systemCharDelta)
			}
			parts = append(parts, fmt.Sprintf("system prompt changed%s", charInfo))
		}
		if changes.toolSchemasChanged {
			toolDiff := ""
			if changes.addedToolCount > 0 || changes.removedToolCount > 0 {
				toolDiff = fmt.Sprintf(" (+%d/-%d tools)", changes.addedToolCount, changes.removedToolCount)
			} else {
				toolDiff = " (tool prompt/schema changed, same tool set)"
			}
			parts = append(parts, fmt.Sprintf("tools changed%s", toolDiff))
		}
		if changes.fastModeChanged {
			parts = append(parts, "fast mode toggled")
		}
		if changes.globalCacheStrategyChanged {
			prevStr := changes.prevGlobalCacheStrategy
			if prevStr == "" {
				prevStr = "none"
			}
			newStr := changes.newGlobalCacheStrategy
			if newStr == "" {
				newStr = "none"
			}
			parts = append(parts, fmt.Sprintf("global cache strategy changed (%s → %s)", prevStr, newStr))
		}
		if changes.cacheControlChanged && !changes.globalCacheStrategyChanged && !changes.systemPromptChanged {
			parts = append(parts, "cache_control changed (scope or TTL)")
		}
		if changes.betasChanged {
			var diffParts []string
			if len(changes.addedBetas) > 0 {
				diffParts = append(diffParts, fmt.Sprintf("+%s", strings.Join(changes.addedBetas, ",")))
			}
			if len(changes.removedBetas) > 0 {
				diffParts = append(diffParts, fmt.Sprintf("-%s", strings.Join(changes.removedBetas, ",")))
			}
			diff := strings.Join(diffParts, " ")
			if diff != "" {
				parts = append(parts, fmt.Sprintf("betas changed (%s)", diff))
			} else {
				parts = append(parts, "betas changed")
			}
		}
		if changes.autoModeChanged {
			parts = append(parts, "auto mode toggled")
		}
		if changes.overageChanged {
			parts = append(parts, "overage state changed (TTL latched, no flip)")
		}
		if changes.cachedMCChanged {
			parts = append(parts, "cached microcompact toggled")
		}
		if changes.effortChanged {
			prevEffort := changes.prevEffortValue
			if prevEffort == "" {
				prevEffort = "default"
			}
			newEffort := changes.newEffortValue
			if newEffort == "" {
				newEffort = "default"
			}
			parts = append(parts, fmt.Sprintf("effort changed (%s → %s)", prevEffort, newEffort))
		}
		if changes.extraBodyChanged {
			parts = append(parts, "extra body params changed")
		}
	}

	var reason string
	if len(parts) > 0 {
		reason = strings.Join(parts, ", ")
	} else {
		reason = "likely server-side (prompt unchanged)"
	}

	// Write diff file for debugging.
	var diffPath string
	if changes != nil && changes.buildPrevDiffableContent != nil {
		prevContent := changes.buildPrevDiffableContent()
		newContent := ""
		if state.buildDiffableContent != nil {
			newContent = state.buildDiffableContent()
		}
		if p, err := writeCacheBreakDiff(prevContent, newContent); err == nil {
			diffPath = p
		}
	}

	d.mu.Unlock()

	var diffSuffix string
	if diffPath != "" {
		diffSuffix = fmt.Sprintf(", diff: %s", diffPath)
	}
	summary := fmt.Sprintf("[PROMPT CACHE BREAK] %s [source=%s, call #%d, cache read: %d → %d, creation: %d%s]",
		reason, source, state.callCount, *prevCacheRead, cacheReadTokens, cacheCreationTokens, diffSuffix)

	logger.WarnCF("prompt_cache", summary, map[string]any{
		"source":              source,
		"call_count":          state.callCount,
		"prev_cache_read":     *prevCacheRead,
		"cache_read":          cacheReadTokens,
		"cache_creation":      cacheCreationTokens,
		"request_id":          requestID,
	})

	d.mu.Lock()
	if s := d.states[key]; s != nil {
		s.pendingChanges = nil
	}
	d.mu.Unlock()
}

// NotifyCacheDeletion marks that a cache deletion is pending, so the next
// drop in cache read tokens is expected rather than a break.
func (d *CacheBreakDetector) NotifyCacheDeletion(source, agentID string) {
	key := getTrackingKey(source, agentID)
	if key == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if state := d.states[key]; state != nil {
		state.cacheDeletionsPending = true
	}
}

// NotifyCompaction resets the cache read baseline after compaction.
func (d *CacheBreakDetector) NotifyCompaction(source, agentID string) {
	key := getTrackingKey(source, agentID)
	if key == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if state := d.states[key]; state != nil {
		state.prevCacheReadTokens = nil
	}
}

// CleanupAgentTracking removes all tracking state for a given agent ID.
func (d *CacheBreakDetector) CleanupAgentTracking(agentID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.states, agentID)
}

// Reset clears all tracking state.
func (d *CacheBreakDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.states = make(map[string]*previousState)
}

// getCacheBreakDiffPath returns a random temp file path for diff output.
func getCacheBreakDiffPath() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	var suffix strings.Builder
	for range 4 {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		suffix.WriteByte(chars[n.Int64()])
	}
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, fmt.Sprintf("cache-break-%s.diff", suffix.String()))
}

// writeCacheBreakDiff writes a unified diff of prev and new content to a temp file.
func writeCacheBreakDiff(prevContent, newContent string) (string, error) {
	diffPath := getCacheBreakDiffPath()
	patch := generateSimpleDiff(prevContent, newContent)
	if err := os.MkdirAll(filepath.Dir(diffPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(diffPath, []byte(patch), 0644); err != nil {
		return "", err
	}
	return diffPath, nil
}

// generateSimpleDiff produces a minimal unified-diff-like patch from two strings.
func generateSimpleDiff(prev, next string) string {
	prevLines := strings.Split(prev, "\n")
	nextLines := strings.Split(next, "\n")

	var out strings.Builder
	out.WriteString("--- before\n")
	out.WriteString("+++ after\n\n")

	maxLen := max(len(prevLines), len(nextLines))

	inHunk := false
	for i := range maxLen {
		var pLine, nLine string
		if i < len(prevLines) {
			pLine = prevLines[i]
		}
		if i < len(nextLines) {
			nLine = nextLines[i]
		}
		if pLine == nLine {
			if inHunk {
				out.WriteString(" " + pLine + "\n")
			}
		} else {
			if !inHunk {
				fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", i+1, 1, i+1, 1)
				inHunk = true
			}
			if i < len(prevLines) {
				out.WriteString("-" + pLine + "\n")
			}
			if i < len(nextLines) {
				out.WriteString("+" + nLine + "\n")
			}
		}
	}
	return out.String()
}
