package anthropic

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

func TestNewCacheBreakDetector(t *testing.T) {
	d := NewCacheBreakDetector()
	if d == nil {
		t.Fatal("NewCacheBreakDetector returned nil")
	}
	if d.states == nil {
		t.Fatal("states map not initialized")
	}
}

func TestGetTrackingKey(t *testing.T) {
	tests := []struct {
		source  string
		agentID string
		want    string
	}{
		{"compact", "", "repl_main_thread"},
		{"repl_main_thread", "", "repl_main_thread"},
		{"repl_main_thread", "agent-123", "agent-123"},
		{"sdk", "", "sdk"},
		{"sdk", "agent-456", "agent-456"},
		{"agent:custom", "my-agent", "my-agent"},
		{"speculation", "", ""},
		{"session_memory", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.source+"_"+tt.agentID, func(t *testing.T) {
			got := getTrackingKey(tt.source, tt.agentID)
			if got != tt.want {
				t.Errorf("getTrackingKey(%q, %q) = %q, want %q", tt.source, tt.agentID, got, tt.want)
			}
		})
	}
}

func TestIsExcludedModel(t *testing.T) {
	if !isExcludedModel("claude-3-haiku-123") {
		t.Error("expected haiku model to be excluded")
	}
	if isExcludedModel("claude-3-sonnet-123") {
		t.Error("expected sonnet model not to be excluded")
	}
}

func TestDjb2HashUint64(t *testing.T) {
	// Hash should be deterministic.
	h1 := djb2HashUint64("hello")
	h2 := djb2HashUint64("hello")
	if h1 != h2 {
		t.Errorf("djb2HashUint64 not deterministic: %d vs %d", h1, h2)
	}
	// Different inputs should yield different hashes (with high probability).
	h3 := djb2HashUint64("world")
	if h1 == h3 {
		t.Error("djb2HashUint64 collision for different inputs")
	}
}

func TestComputeHash(t *testing.T) {
	h1 := computeHash(map[string]any{"a": 1})
	h2 := computeHash(map[string]any{"a": 1})
	if h1 != h2 {
		t.Error("computeHash not deterministic for identical data")
	}
	h3 := computeHash(map[string]any{"a": 2})
	if h1 == h3 {
		t.Error("computeHash should differ for different data")
	}
}

func TestComputePerToolHashes(t *testing.T) {
	tools := []model.ToolDefinition{
		{Name: "tool1", Description: "desc1", InputSchema: map[string]any{"type": "object"}},
		{Name: "tool2", Description: "desc2", InputSchema: map[string]any{"type": "string"}},
	}
	hashes := computePerToolHashes(tools)
	if len(hashes) != 2 {
		t.Fatalf("expected 2 hashes, got %d", len(hashes))
	}
	if _, ok := hashes["tool1"]; !ok {
		t.Error("missing hash for tool1")
	}
	if _, ok := hashes["tool2"]; !ok {
		t.Error("missing hash for tool2")
	}
	// Same tools should produce same hashes.
	hashes2 := computePerToolHashes(tools)
	if hashes["tool1"] != hashes2["tool1"] {
		t.Error("per-tool hash not deterministic")
	}
}

func TestBuildDiffableContent(t *testing.T) {
	tools := []model.ToolDefinition{
		{Name: "z_tool", Description: "z_desc", InputSchema: map[string]any{"type": "object"}},
		{Name: "a_tool", Description: "a_desc", InputSchema: map[string]any{"type": "string"}},
	}
	content := buildDiffableContent("system prompt", tools, "claude-sonnet")
	if !strings.Contains(content, "system prompt") {
		t.Error("diffable content missing system prompt")
	}
	if !strings.Contains(content, "a_tool") {
		t.Error("diffable content missing a_tool")
	}
	if !strings.Contains(content, "z_tool") {
		t.Error("diffable content missing z_tool")
	}
	// Tools should be sorted alphabetically.
	aIdx := strings.Index(content, "a_tool")
	zIdx := strings.Index(content, "z_tool")
	if aIdx == -1 || zIdx == -1 || aIdx > zIdx {
		t.Error("tools not sorted alphabetically in diffable content")
	}
}

func TestRecordPromptState_FirstCall(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System:    "system",
		Tools:     nil,
		Source:    "repl_main_thread",
		Model:     "claude-sonnet",
		FastMode:  false,
	})

	d.mu.RLock()
	defer d.mu.RUnlock()
	state, ok := d.states["repl_main_thread"]
	if !ok {
		t.Fatal("state not recorded for repl_main_thread")
	}
	if state.callCount != 1 {
		t.Errorf("expected callCount=1, got %d", state.callCount)
	}
	if state.pendingChanges != nil {
		t.Error("first call should not have pendingChanges")
	}
}

func TestRecordPromptState_DetectChanges(t *testing.T) {
	d := NewCacheBreakDetector()
	// First call establishes baseline.
	d.RecordPromptState(PromptStateSnapshot{
		System:    "system v1",
		Tools:     []model.ToolDefinition{{Name: "tool1", Description: "desc", InputSchema: map[string]any{}}},
		Source:    "repl_main_thread",
		Model:     "claude-sonnet",
		Betas:     []string{"beta1"},
		FastMode:  false,
	})

	// Second call with changes.
	d.RecordPromptState(PromptStateSnapshot{
		System:    "system v2",
		Tools:     []model.ToolDefinition{{Name: "tool1", Description: "new desc", InputSchema: map[string]any{}}},
		Source:    "repl_main_thread",
		Model:     "claude-opus",
		Betas:     []string{"beta1", "beta2"},
		FastMode:  true,
	})

	d.mu.RLock()
	state := d.states["repl_main_thread"]
	d.mu.RUnlock()

	if state.callCount != 2 {
		t.Errorf("expected callCount=2, got %d", state.callCount)
	}
	if state.pendingChanges == nil {
		t.Fatal("expected pendingChanges after model change")
	}
	if !state.pendingChanges.systemPromptChanged {
		t.Error("expected systemPromptChanged=true")
	}
	if !state.pendingChanges.modelChanged {
		t.Error("expected modelChanged=true")
	}
	if !state.pendingChanges.toolSchemasChanged {
		t.Error("expected toolSchemasChanged=true")
	}
	if !state.pendingChanges.betasChanged {
		t.Error("expected betasChanged=true")
	}
	if !state.pendingChanges.fastModeChanged {
		t.Error("expected fastModeChanged=true")
	}
	if state.pendingChanges.previousModel != "claude-sonnet" {
		t.Errorf("expected previousModel=claude-sonnet, got %s", state.pendingChanges.previousModel)
	}
	if state.pendingChanges.newModel != "claude-opus" {
		t.Errorf("expected newModel=claude-opus, got %s", state.pendingChanges.newModel)
	}
}

func TestRecordPromptState_NoChanges(t *testing.T) {
	d := NewCacheBreakDetector()
	snapshot := PromptStateSnapshot{
		System:   "same system",
		Tools:    []model.ToolDefinition{{Name: "tool1", Description: "desc", InputSchema: map[string]any{}}},
		Source:   "repl_main_thread",
		Model:    "claude-sonnet",
		FastMode: false,
	}
	d.RecordPromptState(snapshot)
	d.RecordPromptState(snapshot)

	d.mu.RLock()
	state := d.states["repl_main_thread"]
	d.mu.RUnlock()

	if state.pendingChanges != nil {
		t.Error("expected no pendingChanges when nothing changed")
	}
	if state.callCount != 2 {
		t.Errorf("expected callCount=2, got %d", state.callCount)
	}
}

func TestRecordPromptState_ToolAddRemove(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System: "sys",
		Tools: []model.ToolDefinition{
			{Name: "keep", Description: "keep", InputSchema: map[string]any{}},
			{Name: "remove", Description: "remove", InputSchema: map[string]any{}},
		},
		Source: "sdk",
		Model:  "claude-sonnet",
	})
	d.RecordPromptState(PromptStateSnapshot{
		System: "sys",
		Tools: []model.ToolDefinition{
			{Name: "keep", Description: "keep", InputSchema: map[string]any{}},
			{Name: "add", Description: "add", InputSchema: map[string]any{}},
		},
		Source: "sdk",
		Model:  "claude-sonnet",
	})

	d.mu.RLock()
	state := d.states["sdk"]
	d.mu.RUnlock()

	if state.pendingChanges == nil {
		t.Fatal("expected pendingChanges")
	}
	if state.pendingChanges.addedToolCount != 1 {
		t.Errorf("expected 1 added tool, got %d", state.pendingChanges.addedToolCount)
	}
	if state.pendingChanges.removedToolCount != 1 {
		t.Errorf("expected 1 removed tool, got %d", state.pendingChanges.removedToolCount)
	}
	if len(state.pendingChanges.addedTools) != 1 || state.pendingChanges.addedTools[0] != "add" {
		t.Errorf("expected addedTools=['add'], got %v", state.pendingChanges.addedTools)
	}
	if len(state.pendingChanges.removedTools) != 1 || state.pendingChanges.removedTools[0] != "remove" {
		t.Errorf("expected removedTools=['remove'], got %v", state.pendingChanges.removedTools)
	}
}

func TestRecordPromptState_MaxTrackedSources(t *testing.T) {
	d := NewCacheBreakDetector()
	// Fill up to maxTrackedSources.
	for i := range maxTrackedSources + 3 {
		d.RecordPromptState(PromptStateSnapshot{
			System:   "sys",
			Source:   "sdk",
			AgentID:  string(rune('a' + i)),
			Model:    "claude-sonnet",
		})
	}

	d.mu.RLock()
	count := len(d.states)
	d.mu.RUnlock()

	if count > maxTrackedSources {
		t.Errorf("expected at most %d tracked sources, got %d", maxTrackedSources, count)
	}
}

func TestCheckResponseForCacheBreak_SkipsFirstCall(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System:   "sys",
		Source:   "repl_main_thread",
		Model:    "claude-sonnet",
		FastMode: false,
	})
	// First response should not trigger a break.
	d.CheckResponseForCacheBreak("repl_main_thread", 1000, 0, nil, "", "req-1")

	d.mu.RLock()
	state := d.states["repl_main_thread"]
	d.mu.RUnlock()

	if state.prevCacheReadTokens == nil || *state.prevCacheReadTokens != 1000 {
		t.Error("expected prevCacheReadTokens to be set to 1000 after first call")
	}
}

func TestCheckResponseForCacheBreak_NoBreak(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System:   "sys",
		Source:   "repl_main_thread",
		Model:    "claude-sonnet",
		FastMode: false,
	})
	// First call establishes baseline.
	d.CheckResponseForCacheBreak("repl_main_thread", 10000, 0, nil, "", "req-1")
	// Small drop (< 5% and < 2000) should not trigger break.
	d.CheckResponseForCacheBreak("repl_main_thread", 9600, 0, nil, "", "req-2")

	// No break should be logged; just verify no panic.
}

func TestCheckResponseForCacheBreak_DetectsBreak(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System:   "sys",
		Source:   "repl_main_thread",
		Model:    "claude-sonnet",
		FastMode: false,
	})
	// First call.
	d.CheckResponseForCacheBreak("repl_main_thread", 10000, 0, nil, "", "req-1")
	// Big drop (>5% and >2000) with no changes should trigger "likely server-side".
	d.CheckResponseForCacheBreak("repl_main_thread", 1000, 5000, nil, "", "req-2")

	// pendingChanges should be cleared after check.
	d.mu.RLock()
	state := d.states["repl_main_thread"]
	d.mu.RUnlock()
	if state.pendingChanges != nil {
		t.Error("expected pendingChanges to be cleared after check")
	}
}

func TestCheckResponseForCacheBreak_WithChanges(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System:   "sys v1",
		Source:   "repl_main_thread",
		Model:    "claude-sonnet",
		FastMode: false,
	})
	// First call.
	d.CheckResponseForCacheBreak("repl_main_thread", 10000, 0, nil, "", "req-1")

	// Change system prompt.
	d.RecordPromptState(PromptStateSnapshot{
		System:   "sys v2",
		Source:   "repl_main_thread",
		Model:    "claude-sonnet",
		FastMode: false,
	})

	// Big drop.
	d.CheckResponseForCacheBreak("repl_main_thread", 1000, 5000, nil, "", "req-2")

	// pendingChanges should be cleared.
	d.mu.RLock()
	state := d.states["repl_main_thread"]
	d.mu.RUnlock()
	if state.pendingChanges != nil {
		t.Error("expected pendingChanges to be cleared after check")
	}
}

func TestCheckResponseForCacheBreak_SkipsExcludedModel(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System:   "sys",
		Source:   "repl_main_thread",
		Model:    "claude-3-haiku-123",
		FastMode: false,
	})
	d.CheckResponseForCacheBreak("repl_main_thread", 10000, 0, nil, "", "req-1")
	d.CheckResponseForCacheBreak("repl_main_thread", 1000, 0, nil, "", "req-2")

	// Should not panic and prevCacheReadTokens should remain nil for excluded models.
	d.mu.RLock()
	state := d.states["repl_main_thread"]
	d.mu.RUnlock()
	if state.prevCacheReadTokens != nil {
		t.Error("expected prevCacheReadTokens to remain nil for excluded model")
	}
}

func TestCheckResponseForCacheBreak_CacheDeletion(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System:   "sys",
		Source:   "repl_main_thread",
		Model:    "claude-sonnet",
		FastMode: false,
	})
	d.CheckResponseForCacheBreak("repl_main_thread", 10000, 0, nil, "", "req-1")
	d.NotifyCacheDeletion("repl_main_thread", "")
	d.CheckResponseForCacheBreak("repl_main_thread", 5000, 0, nil, "", "req-2")

	d.mu.RLock()
	state := d.states["repl_main_thread"]
	d.mu.RUnlock()
	if state.cacheDeletionsPending {
		t.Error("expected cacheDeletionsPending to be cleared")
	}
}

func TestNotifyCompaction(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System:   "sys",
		Source:   "repl_main_thread",
		Model:    "claude-sonnet",
		FastMode: false,
	})
	d.CheckResponseForCacheBreak("repl_main_thread", 10000, 0, nil, "", "req-1")

	// After compaction, prevCacheReadTokens should be reset.
	d.NotifyCompaction("repl_main_thread", "")

	d.mu.RLock()
	state := d.states["repl_main_thread"]
	d.mu.RUnlock()
	if state.prevCacheReadTokens != nil {
		t.Error("expected prevCacheReadTokens to be nil after compaction")
	}
}

func TestCleanupAgentTracking(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System:  "sys",
		Source:  "agent:custom",
		AgentID: "agent-123",
		Model:   "claude-sonnet",
	})
	d.CleanupAgentTracking("agent-123")

	d.mu.RLock()
	_, ok := d.states["agent-123"]
	d.mu.RUnlock()
	if ok {
		t.Error("expected agent-123 state to be removed")
	}
}

func TestReset(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System: "sys",
		Source: "repl_main_thread",
		Model:  "claude-sonnet",
	})
	d.Reset()

	d.mu.RLock()
	count := len(d.states)
	d.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 states after reset, got %d", count)
	}
}

func TestGetCacheBreakDiffPath(t *testing.T) {
	p1 := getCacheBreakDiffPath()
	p2 := getCacheBreakDiffPath()
	if p1 == p2 {
		t.Error("expected different diff paths")
	}
	if !strings.HasSuffix(p1, ".diff") {
		t.Errorf("expected .diff suffix, got %s", p1)
	}
	if !strings.Contains(p1, "cache-break-") {
		t.Errorf("expected 'cache-break-' in path, got %s", p1)
	}
}

func TestWriteCacheBreakDiff(t *testing.T) {
	path, err := writeCacheBreakDiff("line1\nline2", "line1\nline3")
	if err != nil {
		t.Fatalf("writeCacheBreakDiff failed: %v", err)
	}
	defer os.Remove(path)

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read diff file: %v", err)
	}
	if !strings.Contains(string(content), "--- before") {
		t.Error("diff missing '--- before' header")
	}
	if !strings.Contains(string(content), "+++ after") {
		t.Error("diff missing '+++ after' header")
	}
	if !strings.Contains(string(content), "-line2") {
		t.Error("diff missing removed line")
	}
	if !strings.Contains(string(content), "+line3") {
		t.Error("diff missing added line")
	}
}

func TestGenerateSimpleDiff(t *testing.T) {
	patch := generateSimpleDiff("a\nb\nc", "a\nx\nc")
	if !strings.Contains(patch, "-b") {
		t.Error("diff should contain removed line 'b'")
	}
	if !strings.Contains(patch, "+x") {
		t.Error("diff should contain added line 'x'")
	}
	// Unchanged lines may appear inside hunks as context.
	if !strings.Contains(patch, "@@ -2,1 +2,1 @@") {
		t.Error("diff should contain hunk header")
	}
}

func TestRecordPromptState_UntrackedSource(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System: "sys",
		Source: "speculation",
		Model:  "claude-sonnet",
	})

	d.mu.RLock()
	count := len(d.states)
	d.mu.RUnlock()
	if count != 0 {
		t.Error("untracked source should not create state")
	}
}

func TestCheckResponseForCacheBreak_UntrackedSource(t *testing.T) {
	d := NewCacheBreakDetector()
	// Should not panic for untracked source.
	d.CheckResponseForCacheBreak("speculation", 1000, 0, nil, "", "req-1")
}

func TestRecordPromptState_BetaSorting(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System: "sys",
		Source: "sdk",
		Model:  "claude-sonnet",
		Betas:  []string{"zeta", "alpha", "beta"},
	})

	d.mu.RLock()
	state := d.states["sdk"]
	d.mu.RUnlock()

	expected := []string{"alpha", "beta", "zeta"}
	if len(state.betas) != len(expected) {
		t.Fatalf("expected %d betas, got %d", len(expected), len(state.betas))
	}
	for i, b := range expected {
		if state.betas[i] != b {
			t.Errorf("expected betas[%d]=%s, got %s", i, b, state.betas[i])
		}
	}
}

func TestRecordPromptState_CharCount(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System: "hello world",
		Source: "sdk",
		Model:  "claude-sonnet",
	})

	d.mu.RLock()
	state := d.states["sdk"]
	d.mu.RUnlock()

	if state.systemCharCount != 11 {
		t.Errorf("expected systemCharCount=11, got %d", state.systemCharCount)
	}
}

func TestRecordPromptState_ExtraBodyHash(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordPromptState(PromptStateSnapshot{
		System:          "sys",
		Source:          "sdk",
		Model:           "claude-sonnet",
		ExtraBodyParams: map[string]any{"key": "value"},
	})

	d.mu.RLock()
	state1 := d.states["sdk"]
	d.mu.RUnlock()
	if state1.extraBodyHash == 0 {
		t.Error("expected non-zero extraBodyHash")
	}

	// Same extra body params should not trigger change.
	d.RecordPromptState(PromptStateSnapshot{
		System:          "sys",
		Source:          "sdk",
		Model:           "claude-sonnet",
		ExtraBodyParams: map[string]any{"key": "value"},
	})

	d.mu.RLock()
	state2 := d.states["sdk"]
	d.mu.RUnlock()
	if state2.pendingChanges != nil {
		t.Error("same extra body params should not trigger change")
	}

	// Different extra body params should trigger change.
	d.RecordPromptState(PromptStateSnapshot{
		System:          "sys",
		Source:          "sdk",
		Model:           "claude-sonnet",
		ExtraBodyParams: map[string]any{"key": "other"},
	})

	d.mu.RLock()
	state3 := d.states["sdk"]
	d.mu.RUnlock()
	if state3.pendingChanges == nil || !state3.pendingChanges.extraBodyChanged {
		t.Error("different extra body params should trigger change")
	}
}

func TestConcurrentAccess(t *testing.T) {
	d := NewCacheBreakDetector()
	// Concurrent writes should not panic or race.
	for i := range 100 {
		go func(idx int) {
			d.RecordPromptState(PromptStateSnapshot{
				System:  "sys",
				Source:  "sdk",
				AgentID: string(rune('a' + idx%26)),
				Model:   "claude-sonnet",
			})
		}(i)
	}
	for i := range 100 {
		go func(idx int) {
			d.CheckResponseForCacheBreak("sdk", 1000, 0, nil, string(rune('a'+idx%26)), "")
		}(i)
	}
	// Give goroutines time to run.
	time.Sleep(50 * time.Millisecond)
}
