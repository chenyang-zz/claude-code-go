package prompts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScratchpadSection instructs the model to use a session-specific scratchpad directory.
type ScratchpadSection struct{}

// Name returns the section identifier.
func (s ScratchpadSection) Name() string { return "scratchpad" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s ScratchpadSection) IsVolatile() bool { return false }

// Compute generates scratchpad instructions for the current session.
func (s ScratchpadSection) Compute(ctx context.Context) (string, error) {
	data, _ := RuntimeContextFromContext(ctx)
	scratchpadDir, err := scratchpadDir(data.WorkingDir, data.SessionID)
	if err != nil || scratchpadDir == "" {
		return "", nil
	}

	return fmt.Sprintf(`# Scratchpad Directory

IMPORTANT: Always use this scratchpad directory for temporary files instead of %q or other system temp directories:
%q

Use this directory for ALL temporary file needs:
- Storing intermediate results or data during multi-step tasks
- Writing temporary scripts or configuration files
- Saving outputs that don't belong in the user's project
- Creating working files during analysis or processing
- Any file that would otherwise go to %q

Only use %q if the user explicitly requests it.

The scratchpad directory is session-specific, isolated from the user's project, and can be used freely without permission prompts.`, os.TempDir(), scratchpadDir, os.TempDir(), os.TempDir()), nil
}

// scratchpadDir derives a stable, session-scoped scratchpad directory path.
func scratchpadDir(workingDir, sessionID string) (string, error) {
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return "", nil
	}
	cwd := strings.TrimSpace(workingDir)
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	sum := sha256.Sum256([]byte(filepath.Clean(cwd)))
	scope := hex.EncodeToString(sum[:8])
	dir := filepath.Join(os.TempDir(), "claude-code-go", scope, trimmedSessionID, "scratchpad")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
