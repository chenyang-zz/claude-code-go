package coordinator

import (
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SessionMode represents the coordinator mode stored in a session snapshot.
type SessionMode string

const (
	SessionModeCoordinator SessionMode = "coordinator"
	SessionModeNormal      SessionMode = "normal"
	SessionModeUnset       SessionMode = ""
)

// MatchSessionMode checks whether the current coordinator mode matches the
// session's stored mode. If there is a mismatch, it flips the environment
// variable so IsCoordinatorMode() returns the correct value for the resumed
// session. Returns a warning message if the mode was switched, or empty string
// if no switch was needed.
//
// Corresponds to TS matchSessionMode() in src/coordinator/coordinatorMode.ts.
func MatchSessionMode(sessionMode SessionMode) string {
	if sessionMode == SessionModeUnset {
		return ""
	}

	currentIsCoordinator := IsCoordinatorMode()
	sessionIsCoordinator := sessionMode == SessionModeCoordinator

	if currentIsCoordinator == sessionIsCoordinator {
		return ""
	}

	// Flip the env var — IsCoordinatorMode() reads it live, no caching.
	if sessionIsCoordinator {
		os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")
	} else {
		os.Unsetenv("CLAUDE_CODE_COORDINATOR_MODE")
	}

	logger.InfoCF("coordinator", "mode switched to match resumed session", map[string]any{
		"to": string(sessionMode),
	})

	if sessionIsCoordinator {
		return "Entered coordinator mode to match resumed session."
	}
	return "Exited coordinator mode to match resumed session."
}

// ParseSessionMode converts a raw string into a SessionMode value.
func ParseSessionMode(raw string) SessionMode {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "coordinator":
		return SessionModeCoordinator
	case "normal":
		return SessionModeNormal
	default:
		return SessionModeUnset
	}
}
