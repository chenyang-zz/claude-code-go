package teammemsync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// HashContent computes the SHA-256 hash of the given content string.
// Returns "sha256:<hex>" to match the server's entryChecksums format.
func HashContent(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// ComputeDelta returns a map of entries whose local content hash differs from
// the server's known checksums. Keys present locally but not in serverChecksums
// (never synced before) are always included. Keys in serverChecksums but not
// locally are excluded (deletions do not propagate).
func ComputeDelta(
	entries map[string]string,
	serverChecksums map[string]string,
) map[string]string {
	delta := make(map[string]string)
	for key, content := range entries {
		localHash := HashContent(content)
		if serverHash, ok := serverChecksums[key]; !ok || serverHash != localHash {
			delta[key] = content
		}
	}
	return delta
}

// BatchDeltaByBytes splits a delta map into PUT-sized batches, each under
// MaxPutBodyBytes. Keys are sorted for deterministic batching across calls.
// Uses greedy bin-packing. A single entry exceeding the cap gets its own batch
// (MaxFileSizeBytes already caps individual files at 250KB).
func BatchDeltaByBytes(delta map[string]string) []map[string]string {
	keys := make([]string, 0, len(delta))
	for k := range delta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		return nil
	}

	// Fixed overhead for {"entries":{}} — each entry adds its marginal bytes.
	const emptyBodyBytes = len(`{"entries":{}}`)

	entryBytes := func(k, v string) int {
		// json.Marshal handles escaping so the byte count matches what the
		// HTTP client serializes.
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(v)
		return len(kb) + len(vb) + 2 // colon + comma (comma over-counts last entry; harmless slack)
	}

	var batches []map[string]string
	current := make(map[string]string)
	currentBytes := emptyBodyBytes

	for _, key := range keys {
		added := entryBytes(key, delta[key])
		if currentBytes+added > MaxPutBodyBytes && len(current) > 0 {
			batches = append(batches, current)
			current = make(map[string]string)
			currentBytes = emptyBodyBytes
		}
		current[key] = delta[key]
		currentBytes += added
	}
	batches = append(batches, current)
	return batches
}

// ReadLocalTeamMemory reads all team memory files from the local directory
// into a flat key-value map. Keys are relative paths from the team memory
// directory. Oversized files (> MaxFileSizeBytes) are skipped.
// Phase 1: no secret scanning is performed.
func ReadLocalTeamMemory(projectRoot string, maxEntries *int) (entries map[string]string, err error) {
	teamDir := GetTeamMemPath(projectRoot)
	entries = make(map[string]string)

	err = walkTeamDir(teamDir, teamDir, entries)
	if err != nil {
		return nil, err
	}

	// Sort keys for deterministic truncation.
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Truncate if we've learned a server cap.
	if maxEntries != nil && len(keys) > *maxEntries {
		truncated := make(map[string]string)
		for _, key := range keys[:*maxEntries] {
			truncated[key] = entries[key]
		}
		return truncated, nil
	}

	return entries, nil
}
