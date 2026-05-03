// Package teammemsync implements the Team Memory Sync service, which synchronizes
// team memory files between the local filesystem and the Claude.ai backend API.
// Team memory is scoped per-repo (identified by git remote) and shared across all
// authenticated organization members.
package teammemsync

// TeamMemoryContent holds the flat key-value entries that form the team memory
// content payload. Mirrors TS TeamMemoryContentSchema.
type TeamMemoryContent struct {
	// Entries maps relative file paths to their UTF-8 string content.
	Entries map[string]string `json:"entries"`
	// EntryChecksums holds per-key SHA-256 hashes (format: "sha256:<hex>").
	// Added in anthropic/anthropic#283027. Optional for backward compatibility.
	EntryChecksums map[string]string `json:"entryChecksums,omitempty"`
}

// TeamMemoryData is the full GET /api/claude_code/team_memory response body.
// Mirrors TS TeamMemoryDataSchema.
type TeamMemoryData struct {
	OrganizationID string             `json:"organizationId"`
	Repo           string             `json:"repo"`
	Version        int                `json:"version"`
	LastModified   string             `json:"lastModified"`
	Checksum       string             `json:"checksum"`
	Content        TeamMemoryContent  `json:"content"`
}

// TeamMemoryTooManyEntries is the structured 413 error body from the server when
// the entry count exceeds the per-org cap. Mirrors TS TeamMemoryTooManyEntriesSchema.
type TeamMemoryTooManyEntries struct {
	Error struct {
		Details struct {
			ErrorCode        string `json:"error_code"`
			MaxEntries       int    `json:"max_entries"`
			ReceivedEntries  int    `json:"received_entries"`
		} `json:"details"`
	} `json:"error"`
}

// SkippedSecretFile records a file that was excluded from push because it
// contained a detected secret. The secret value is never stored.
// Phase 1: type placeholder only; no secret scanning is performed yet.
type SkippedSecretFile struct {
	Path   string `json:"path"`
	RuleID string `json:"ruleId"`
	Label  string `json:"label"`
}

// SyncState holds the mutable state for the team memory sync service.
// Created once per session and threaded through all sync operations.
// Tests create a fresh instance per test for isolation.
// Mirrors TS SyncState type.
type SyncState struct {
	// LastKnownChecksum is the server-side checksum (ETag) used for conditional
	// GET (If-None-Match) and optimistic PUT (If-Match).
	LastKnownChecksum string

	// ServerChecksums maps relative file paths to their SHA-256 content hash
	// ("sha256:<hex>") as reported by the server. Used to compute the push delta —
	// only keys whose local hash differs are uploaded.
	ServerChecksums map[string]string

	// ServerMaxEntries is the server-enforced max_entries cap, learned from a
	// structured 413 response. Nil until a 413 is observed.
	ServerMaxEntries *int
}

// NewSyncState creates a new SyncState with empty maps.
func NewSyncState() *SyncState {
	return &SyncState{
		ServerChecksums: make(map[string]string),
	}
}

// FetchResult is the outcome of a team memory fetch (pull) attempt.
// Mirrors TS TeamMemorySyncFetchResult.
type FetchResult struct {
	Success     bool
	Data        *TeamMemoryData
	IsEmpty     bool // true if 404 (no data exists)
	NotModified bool // true if 304 (ETag matched, no changes)
	Checksum    string
	Error       string
	SkipRetry   bool
	ErrorType   string // auth, timeout, network, parse, unknown
	HTTPStatus  int
}

// HashesResult is the outcome of a lightweight GET ?view=hashes probe.
// Contains per-key checksums without entry bodies. Used for cheap
// serverChecksums refresh during 412 conflict resolution.
// Mirrors TS TeamMemoryHashesResult.
type HashesResult struct {
	Success        bool
	Version        int
	Checksum       string
	EntryChecksums map[string]string
	Error          string
	ErrorType      string // auth, timeout, network, parse, unknown
	HTTPStatus     int
}

// PushResult is the outcome of a team memory push attempt.
// Mirrors TS TeamMemorySyncPushResult.
type PushResult struct {
	Success        bool
	FilesUploaded  int
	Checksum       string
	Conflict       bool // true if 412 Precondition Failed
	Error          string
	SkippedSecrets []SkippedSecretFile
	ErrorType      string // auth, timeout, network, conflict, unknown, no_oauth, no_repo
	HTTPStatus     int
}

// UploadResult is the outcome of a single upload (PUT) attempt.
// Mirrors TS TeamMemorySyncUploadResult.
type UploadResult struct {
	Success              bool
	Checksum             string
	LastModified         string
	Conflict             bool // true if 412
	Error                string
	ErrorType            string // auth, timeout, network, unknown
	HTTPStatus           int
	ServerErrorCode      string // "team_memory_too_many_entries"
	ServerMaxEntries     int
	ServerReceivedEntries int
}

const (
	// DefaultBaseAPIURL is the base URL for the team memory sync API.
	DefaultBaseAPIURL = "https://api.anthropic.com"
	// TeamMemorySyncTimeout is the HTTP client timeout for sync requests.
	TeamMemorySyncTimeout = 30 * 1000 // ms, mirrored as Duration below
	// MaxFileSizeBytes is the per-entry size cap. Matches TS MAX_FILE_SIZE_BYTES.
	MaxFileSizeBytes = 250_000
	// MaxPutBodyBytes is the PUT body size cap for batching.
	// 200KB leaves headroom under the gateway's observed ~256KB threshold.
	MaxPutBodyBytes = 200_000
	// DefaultMaxRetries is the number of retries for fetch operations.
	DefaultMaxRetries = 3
	// MaxConflictRetries is the number of 412 conflict resolution retries.
	MaxConflictRetries = 2
)
