package teammemsync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ─── Types ─────────────────────────────────────────────────────

func TestNewSyncState(t *testing.T) {
	state := NewSyncState()
	if state == nil {
		t.Fatal("NewSyncState returned nil")
	}
	if state.ServerChecksums == nil {
		t.Error("ServerChecksums map not initialised")
	}
	if state.LastKnownChecksum != "" {
		t.Errorf("expected empty LastKnownChecksum, got %q", state.LastKnownChecksum)
	}
	if state.ServerMaxEntries != nil {
		t.Error("expected nil ServerMaxEntries")
	}
}

func TestTeamMemoryDataJSONRoundTrip(t *testing.T) {
	data := TeamMemoryData{
		OrganizationID: "org_123",
		Repo:           "owner/repo",
		Version:        1,
		LastModified:   "2026-01-01T00:00:00Z",
		Checksum:       "sha256:abc123",
		Content: TeamMemoryContent{
			Entries: map[string]string{
				"MEMORY.md":  "# Team Memory",
				"patterns.md": "## Patterns",
			},
			EntryChecksums: map[string]string{
				"MEMORY.md":  "sha256:def456",
				"patterns.md": "sha256:ghi789",
			},
		},
	}

	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded TeamMemoryData
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.Repo != data.Repo {
		t.Errorf("repo mismatch: %q != %q", decoded.Repo, data.Repo)
	}
	if len(decoded.Content.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(decoded.Content.Entries))
	}
}

func TestTeamMemoryTooManyEntriesJSON(t *testing.T) {
	body := `{"error":{"details":{"error_code":"team_memory_too_many_entries","max_entries":50,"received_entries":75}}}`
	var tooMany TeamMemoryTooManyEntries
	if err := json.Unmarshal([]byte(body), &tooMany); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if tooMany.Error.Details.ErrorCode != "team_memory_too_many_entries" {
		t.Errorf("error_code mismatch: %q", tooMany.Error.Details.ErrorCode)
	}
	if tooMany.Error.Details.MaxEntries != 50 {
		t.Errorf("max_entries mismatch: %d", tooMany.Error.Details.MaxEntries)
	}
	if tooMany.Error.Details.ReceivedEntries != 75 {
		t.Errorf("received_entries mismatch: %d", tooMany.Error.Details.ReceivedEntries)
	}
}

// ─── Hash ───────────────────────────────────────────────────────

func TestHashContent(t *testing.T) {
	h := HashContent("hello")
	if !strings.HasPrefix(h, "sha256:") {
		t.Errorf("hash should have sha256: prefix, got %q", h)
	}
	// SHA-256 of "hello" = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	expected := "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if h != expected {
		t.Errorf("hash mismatch: got %q, want %q", h, expected)
	}
}

func TestHashContentDeterministic(t *testing.T) {
	h1 := HashContent("same content")
	h2 := HashContent("same content")
	if h1 != h2 {
		t.Error("hash should be deterministic")
	}
}

func TestHashContentDifferent(t *testing.T) {
	h1 := HashContent("content A")
	h2 := HashContent("content B")
	if h1 == h2 {
		t.Error("different content should produce different hashes")
	}
}

func TestComputeDelta(t *testing.T) {
	entries := map[string]string{
		"a.md": "content A",
		"b.md": "content B new",
		"c.md": "content C",
	}
	serverChecksums := map[string]string{
		"a.md": HashContent("content A"),        // unchanged
		"b.md": HashContent("content B old"),     // changed
		// c.md not in serverChecksums — new entry
	}

	delta := ComputeDelta(entries, serverChecksums)
	if len(delta) != 2 {
		t.Errorf("expected 2 delta entries, got %d: %v", len(delta), delta)
	}
	if _, ok := delta["a.md"]; ok {
		t.Error("a.md should not be in delta (unchanged)")
	}
	if _, ok := delta["b.md"]; !ok {
		t.Error("b.md should be in delta (changed)")
	}
	if _, ok := delta["c.md"]; !ok {
		t.Error("c.md should be in delta (new)")
	}
}

func TestComputeDeltaEmpty(t *testing.T) {
	delta := ComputeDelta(map[string]string{}, map[string]string{})
	if len(delta) != 0 {
		t.Errorf("expected empty delta, got %d entries", len(delta))
	}
}

func TestBatchDeltaByBytes(t *testing.T) {
	// Each entry: 7-byte key (f.md) + 60-byte value + json overhead.
	// 200,000 / 80 ~ 2500 entries. Use 3000 unique entries to guarantee at least 2 batches.
	delta := make(map[string]string)
	for i := 0; i < 3000; i++ {
		key := "f" + strconv.Itoa(i) + ".md"
		delta[key] = strings.Repeat("a", 60)
	}

	batches := BatchDeltaByBytes(delta)
	if len(batches) < 2 {
		t.Errorf("expected at least 2 batches for 3000 entries, got %d", len(batches))
	}

	// Verify all entries are present across batches.
	var totalKeys int
	for _, batch := range batches {
		totalKeys += len(batch)
	}
	if totalKeys != len(delta) {
		t.Errorf("total keys across batches: %d, expected: %d", totalKeys, len(delta))
	}
}

func TestBatchDeltaByBytesEmpty(t *testing.T) {
	batches := BatchDeltaByBytes(map[string]string{})
	if len(batches) != 0 {
		t.Errorf("expected 0 batches, got %d", len(batches))
	}
}

func TestBatchDeltaByBytesSingleEntry(t *testing.T) {
	delta := map[string]string{"single.md": "hello"}
	batches := BatchDeltaByBytes(delta)
	if len(batches) != 1 {
		t.Errorf("expected 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 1 {
		t.Errorf("expected 1 entry in batch, got %d", len(batches[0]))
	}
}

// ─── Paths ──────────────────────────────────────────────────────

func TestSanitizePathKeyNullByte(t *testing.T) {
	err := sanitizePathKey("file\x00.md")
	if err == nil {
		t.Fatal("expected error for null byte")
	}
	if _, ok := err.(*PathTraversalError); !ok {
		t.Errorf("expected PathTraversalError, got %T", err)
	}
}

func TestSanitizePathKeyBackslash(t *testing.T) {
	err := sanitizePathKey("..\\..\\etc\\passwd")
	if err == nil {
		t.Fatal("expected error for backslash")
	}
}

func TestSanitizePathKeyAbsolute(t *testing.T) {
	err := sanitizePathKey("/etc/passwd")
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestSanitizePathKeyURLEncodedTraversal(t *testing.T) {
	err := sanitizePathKey("%2e%2e%2fetc%2fpasswd")
	if err == nil {
		t.Fatal("expected error for URL-encoded traversal")
	}
}

func TestSanitizePathKeyValid(t *testing.T) {
	tests := []string{
		"MEMORY.md",
		"subdir/file.md",
		"deep/nested/path/doc.md",
		"file-with-dashes.md",
		"file_with_underscores.md",
	}
	for _, key := range tests {
		if err := sanitizePathKey(key); err != nil {
			t.Errorf("unexpected error for %q: %v", key, err)
		}
	}
}

func TestSanitizePathKeyFullwidthTraversal(t *testing.T) {
	// Fullwidth dot and slash characters
	err := sanitizePathKey("．．／etc／passwd")
	if err == nil {
		t.Fatal("expected error for fullwidth traversal")
	}
}

func TestPathTraversalError(t *testing.T) {
	e := &PathTraversalError{Message: "test"}
	if e.Error() != "test" {
		t.Errorf("wrong error message: %q", e.Error())
	}
}

func TestGetTeamMemPath(t *testing.T) {
	path := GetTeamMemPath("/test/project")
	if !strings.HasSuffix(path, string(filepath.Separator)) {
		t.Error("team mem path should have trailing separator")
	}
	if !strings.Contains(path, "team") {
		t.Error("team mem path should contain 'team'")
	}
}

func TestGetTeamMemEntrypoint(t *testing.T) {
	path := GetTeamMemEntrypoint("/test/project")
	if !strings.HasSuffix(path, "MEMORY.md") {
		t.Errorf("entrypoint should end with MEMORY.md, got %q", path)
	}
}

func TestIsTeamMemPath(t *testing.T) {
	projectRoot := "/test/project"
	teamDir := GetTeamMemPath(projectRoot)

	// File inside team dir.
	inside := filepath.Join(teamDir, "MEMORY.md")
	if !IsTeamMemPath(inside, projectRoot) {
		t.Error("file inside team dir should be recognised")
	}

	// File outside team dir.
	outside := "/etc/passwd"
	if IsTeamMemPath(outside, projectRoot) {
		t.Error("file outside team dir should not be recognised")
	}
}

func TestValidateTeamMemKeyValid(t *testing.T) {
	projectRoot := t.TempDir()
	teamDir := GetTeamMemPath(projectRoot)

	// Create the team memory directory so symlink checks pass.
	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		t.Fatalf("failed to create team dir: %v", err)
	}

	resolved, err := ValidateTeamMemKey("MEMORY.md", projectRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resolved, "MEMORY.md") {
		t.Errorf("resolved path should contain MEMORY.md, got %q", resolved)
	}
}

func TestValidateTeamMemKeyTraversal(t *testing.T) {
	projectRoot := t.TempDir()
	_, err := ValidateTeamMemKey("../../etc/passwd", projectRoot)
	if err == nil {
		t.Fatal("expected error for traversal")
	}
}

func TestValidateTeamMemKeyNullByte(t *testing.T) {
	projectRoot := t.TempDir()
	_, err := ValidateTeamMemKey("file\x00.md", projectRoot)
	if err == nil {
		t.Fatal("expected error for null byte")
	}
}

// ─── Pull / Push (HTTP mock) ────────────────────────────────────

func setupMockServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server, server.URL
}

func TestFetchTeamMemoryOnce200(t *testing.T) {
	_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		resp := TeamMemoryData{
			OrganizationID: "org_1",
			Repo:           "owner/repo",
			Version:        1,
			Checksum:       "sha256:etag123",
			Content: TeamMemoryContent{
				Entries:        map[string]string{"MEMORY.md": "# content"},
				EntryChecksums: map[string]string{"MEMORY.md": "sha256:abc"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	result := fetchTeamMemoryOnce(context.Background(), url, "owner/repo", "token-xxx", "")
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Data == nil {
		t.Fatal("expected data, got nil")
	}
	if result.IsEmpty {
		t.Error("expected IsEmpty=false")
	}
	if len(result.Data.Content.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result.Data.Content.Entries))
	}
}

func TestFetchTeamMemoryOnce304(t *testing.T) {
	_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == "" {
			t.Error("expected If-None-Match header for conditional request")
		}
		w.WriteHeader(http.StatusNotModified)
	})

	result := fetchTeamMemoryOnce(context.Background(), url, "owner/repo", "token-xxx", "etag-123")
	if !result.Success {
		t.Errorf("expected success for 304, got error: %s", result.Error)
	}
	if !result.NotModified {
		t.Error("expected NotModified=true for 304")
	}
}

func TestFetchTeamMemoryOnce404(t *testing.T) {
	_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	result := fetchTeamMemoryOnce(context.Background(), url, "owner/repo", "token-xxx", "")
	if !result.Success {
		t.Errorf("expected success for 404, got error: %s", result.Error)
	}
	if !result.IsEmpty {
		t.Error("expected IsEmpty=true for 404")
	}
}

func TestFetchTeamMemoryOnce401(t *testing.T) {
	_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	result := fetchTeamMemoryOnce(context.Background(), url, "owner/repo", "token-xxx", "")
	if result.Success {
		t.Error("expected failure for 401")
	}
	if !result.SkipRetry {
		t.Error("expected SkipRetry=true for auth error")
	}
}

func TestUploadTeamMemoryOnce200(t *testing.T) {
	_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing Content-Type header")
		}
		resp := map[string]string{
			"checksum":     "sha256:new123",
			"lastModified": "2026-01-01T00:00:01Z",
		}
		json.NewEncoder(w).Encode(resp)
	})

	result := uploadTeamMemoryOnce(context.Background(), url, "owner/repo", "token-xxx",
		map[string]string{"MEMORY.md": "# updated"}, "etag-old")
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Checksum != "sha256:new123" {
		t.Errorf("checksum mismatch: %q", result.Checksum)
	}
}

func TestUploadTeamMemoryOnce412(t *testing.T) {
	_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
	})

	result := uploadTeamMemoryOnce(context.Background(), url, "owner/repo", "token-xxx",
		map[string]string{"MEMORY.md": "# updated"}, "etag-old")
	if result.Success {
		t.Error("expected failure for 412")
	}
	if !result.Conflict {
		t.Error("expected Conflict=true for 412")
	}
}

func TestUploadTeamMemoryOnce413Structured(t *testing.T) {
	_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		resp := `{"error":{"details":{"error_code":"team_memory_too_many_entries","max_entries":50,"received_entries":75}}}`
		w.Write([]byte(resp))
	})

	result := uploadTeamMemoryOnce(context.Background(), url, "owner/repo", "token-xxx",
		map[string]string{"many": "entries"}, "")
	if result.Success {
		t.Error("expected failure for 413")
	}
	if result.ServerErrorCode != "team_memory_too_many_entries" {
		t.Errorf("expected team_memory_too_many_entries, got %q", result.ServerErrorCode)
	}
	if result.ServerMaxEntries != 50 {
		t.Errorf("expected ServerMaxEntries=50, got %d", result.ServerMaxEntries)
	}
}

func TestFetchTeamMemoryHashes(t *testing.T) {
	_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "view=hashes") {
			t.Error("expected ?view=hashes parameter")
		}
		resp := map[string]any{
			"checksum": "sha256:hash123",
			"version":  2,
			"entryChecksums": map[string]string{
				"MEMORY.md": "sha256:abc",
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	result := fetchTeamMemoryHashes(context.Background(), url, "owner/repo", "token-xxx")
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if len(result.EntryChecksums) != 1 {
		t.Errorf("expected 1 entry checksum, got %d", len(result.EntryChecksums))
	}
}

// ─── Sync ───────────────────────────────────────────────────────

func TestIsTeamMemorySyncAvailable(t *testing.T) {
	// Without feature flag enabled and no repo, should be false.
	if IsTeamMemorySyncAvailable("") {
		t.Error("should be unavailable without repo slug")
	}
	// With repo slug but without feature flag, depends on env.
	// This test verifies the non-crash path.
	_ = IsTeamMemorySyncAvailable("owner/repo")
}

// ─── ReadLocalTeamMemory ────────────────────────────────────────

func TestReadLocalTeamMemory(t *testing.T) {
	projectRoot := t.TempDir()
	teamDir := GetTeamMemPath(projectRoot)
	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		t.Fatalf("failed to create team dir: %v", err)
	}

	// Write some test files.
	files := map[string]string{
		"MEMORY.md":        "# Team Memory",
		"patterns.md":      "## Patterns",
		"subdir/nested.md": "### Nested Content",
	}
	for path, content := range files {
		fullPath := filepath.Join(teamDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	entries, err := ReadLocalTeamMemory(projectRoot, nil)
	if err != nil {
		t.Fatalf("ReadLocalTeamMemory failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d: %v", len(entries), entries)
	}

	// Check content correctness.
	if entries["MEMORY.md"] != "# Team Memory" {
		t.Errorf("content mismatch for MEMORY.md")
	}
	if entries["subdir/nested.md"] != "### Nested Content" {
		t.Errorf("content mismatch for nested.md")
	}
}

func TestReadLocalTeamMemoryEmpty(t *testing.T) {
	projectRoot := t.TempDir()
	// Don't create team dir — should return empty.
	entries, err := ReadLocalTeamMemory(projectRoot, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries from non-existent dir, got %d", len(entries))
	}
}

func TestReadLocalTeamMemoryTruncation(t *testing.T) {
	projectRoot := t.TempDir()
	teamDir := GetTeamMemPath(projectRoot)
	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	for i := 0; i < 10; i++ {
		name := "file_" + string(rune('a'+i)) + ".md"
		if err := os.WriteFile(filepath.Join(teamDir, name), []byte("content"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	max := 3
	entries, err := ReadLocalTeamMemory(projectRoot, &max)
	if err != nil {
		t.Fatalf("ReadLocalTeamMemory failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 truncated entries, got %d", len(entries))
	}
}

// ─── Exponential Backoff ────────────────────────────────────────

func TestExponentialBackoff(t *testing.T) {
	for attempt := 0; attempt < 5; attempt++ {
		delay := exponentialBackoff(attempt)
		if delay <= 0 {
			t.Errorf("attempt %d: expected positive delay, got %v", attempt, delay)
		}
		if delay > 60*time.Second {
			t.Errorf("attempt %d: delay too large: %v", attempt, delay)
		}
		// Verify exponential growth.
		nextDelay := exponentialBackoff(attempt + 1)
		if nextDelay < delay {
			t.Errorf("delay should grow: %v -> %v", delay, nextDelay)
		}
	}
}

// ─── BatchDeltaByBytes Determinism ──────────────────────────────

func TestBatchDeltaByBytesDeterministic(t *testing.T) {
	delta := map[string]string{
		"z.md": "content z",
		"a.md": "content a",
		"m.md": "content m",
	}

	batches1 := BatchDeltaByBytes(delta)
	batches2 := BatchDeltaByBytes(delta)

	if len(batches1) != len(batches2) {
		t.Errorf("batch count differs: %d vs %d", len(batches1), len(batches2))
	}
	for i := range batches1 {
		keys1 := sortedKeys(batches1[i])
		keys2 := sortedKeys(batches2[i])
		if !stringSlicesEqual(keys1, keys2) {
			t.Errorf("batch %d keys differ", i)
		}
	}
}

// ─── Helpers ────────────────────────────────────────────────────

func TestClassifyHTTPErr(t *testing.T) {
	if classifyHTTPErr(nil) != "" {
		t.Error("nil error should return empty string")
	}
	if kind := classifyHTTPErr(&fakeErr{msg: "timeout"}); kind != "timeout" {
		t.Errorf("timeout error classified as %q", kind)
	}
	if kind := classifyHTTPErr(&fakeErr{msg: "connection refused"}); kind != "network" {
		t.Errorf("network error classified as %q", kind)
	}
}

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{`"hello"`, "hello"},
		{`hello`, "hello"},
		{`""`, ""},
		{"", ""},
	}
	for _, tc := range tests {
		if got := trimQuotes(tc.input); got != tc.expected {
			t.Errorf("trimQuotes(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestURLDecodeIfPossible(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"hello", "hello"},
		{"hello%20world", "hello world"},
		{"%2e%2e%2f", "../"},
		{"no percent", "no percent"},
	}
	for _, tc := range tests {
		decoded, err := urlDecodeIfPossible(tc.input)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", tc.input, err)
		}
		if decoded != tc.expected {
			t.Errorf("urlDecodeIfPossible(%q) = %q, want %q", tc.input, decoded, tc.expected)
		}
	}
}

// ─── End-to-end Pull + Sync via mock ────────────────────────────

func TestPullTeamMemoryIntegration(t *testing.T) {
	_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "view=hashes") {
			json.NewEncoder(w).Encode(map[string]any{
				"entryChecksums": map[string]string{"MEMORY.md": "sha256:abc"},
			})
			return
		}
		resp := TeamMemoryData{
			OrganizationID: "org_1",
			Repo:           "owner/repo",
			Version:        1,
			Checksum:       "sha256:etag123",
			Content: TeamMemoryContent{
				Entries:        map[string]string{"MEMORY.md": "# Team Memory Content"},
				EntryChecksums: map[string]string{"MEMORY.md": "sha256:abc"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	projectRoot := t.TempDir()
	teamDir := GetTeamMemPath(projectRoot)
	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	state := NewSyncState()
	ok, written, count, _, errStr := PullTeamMemory(
		context.Background(), state, url, "owner/repo", "token-xxx", projectRoot, true,
	)
	if !ok {
		t.Fatalf("PullTeamMemory failed: %s", errStr)
	}
	if written != 1 {
		t.Errorf("expected 1 file written, got %d", written)
	}
	if count != 1 {
		t.Errorf("expected 1 entry count, got %d", count)
	}

	// Verify file was written to disk.
	content, err := os.ReadFile(filepath.Join(teamDir, "MEMORY.md"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(content) != "# Team Memory Content" {
		t.Errorf("content mismatch: %q", string(content))
	}
}

func TestPushTeamMemoryIntegration(t *testing.T) {
	_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "view=hashes") {
			json.NewEncoder(w).Encode(map[string]any{
				"entryChecksums": map[string]string{},
			})
			return
		}
		resp := map[string]string{"checksum": "sha256:new-etag", "lastModified": "2026-01-01T00:00:01Z"}
		json.NewEncoder(w).Encode(resp)
	})

	projectRoot := t.TempDir()
	teamDir := GetTeamMemPath(projectRoot)
	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamDir, "test.md"), []byte("test content"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	state := NewSyncState()
	result := PushTeamMemory(context.Background(), state, url, "owner/repo", "token-xxx", projectRoot)
	if !result.Success {
		t.Fatalf("PushTeamMemory failed: %s", result.Error)
	}
	if result.FilesUploaded != 1 {
		t.Errorf("expected 1 file uploaded, got %d", result.FilesUploaded)
	}
}

// ─── SyncState Isolation ────────────────────────────────────────

func TestSyncStateIsolation(t *testing.T) {
	s1 := NewSyncState()
	s2 := NewSyncState()

	s1.ServerChecksums["test.md"] = "sha256:abc"
	if _, ok := s2.ServerChecksums["test.md"]; ok {
		t.Error("SyncState instances should be isolated")
	}
}

// ─── Helper Types ───────────────────────────────────────────────

type fakeErr struct{ msg string }

func (e *fakeErr) Error() string { return e.msg }

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
