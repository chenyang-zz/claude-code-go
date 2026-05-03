// Package mailbox provides a file-based messaging system for agent swarms.
// Each teammate has an inbox file at ~/.claude/teams/{team_name}/inboxes/{agent_name}.json
// that other teammates can write messages to. Inboxes are keyed by agent name within a team.
// All write operations are protected by file locking (flock) for multi-process safety.
package mailbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
)

var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// sanitizeName rewrites a string so it is safe to use as a file or directory name.
// Characters outside [a-zA-Z0-9_-] are replaced with hyphens.
func sanitizeName(input string) string {
	return sanitizeRe.ReplaceAllString(input, "-")
}

// globalMu serializes in-process mailbox operations so concurrent goroutines don't
// clobber each other. Cross-process safety is handled by flock.
var globalMu sync.Mutex

// Message represents a single message in a teammate's inbox.
type Message struct {
	// From is the sender's agent name.
	From string `json:"from"`
	// Text is the message content.
	Text string `json:"text"`
	// Timestamp is the ISO-8601 time when the message was sent.
	Timestamp string `json:"timestamp"`
	// Read indicates whether the recipient has seen this message.
	Read bool `json:"read"`
	// Color is the sender's assigned color (e.g. "red", "blue", "green").
	Color string `json:"color,omitempty"`
	// Summary is a 5-10 word preview shown in the UI.
	Summary string `json:"summary,omitempty"`
}

// getInboxPath returns the file path for a teammate's inbox.
// Structure: {homeDir}/.claude/teams/{team}/inboxes/{agent}.json
func getInboxPath(agentName, teamName, homeDir string) string {
	team := teamName
	if team == "" {
		team = "default"
	}
	safeTeam := sanitizeName(team)
	safeAgent := sanitizeName(agentName)
	return filepath.Join(homeDir, ".claude", "teams", safeTeam, "inboxes", safeAgent+".json")
}

// getLockPath returns the path to the lock file for an inbox.
func getLockPath(inboxPath string) string {
	return inboxPath + ".lock"
}

// ensureInboxDir creates the inbox directory and all parent directories if they
// don't exist.
func ensureInboxDir(agentName, teamName, homeDir string) error {
	path := getInboxPath(agentName, teamName, homeDir)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mailbox: create inbox dir: %w", err)
	}
	return nil
}

// withLock acquires an exclusive flock on the mailbox lock file, calls fn,
// and releases the lock. The lock is held for the entire duration of fn,
// serializing all writes across processes and in-process goroutines.
func withLock(lockPath string, fn func() error) error {
	// Ensure the lock file exists so we can flock it.
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("mailbox: create lock file: %w", err)
	}
	f.Close()

	f, err = os.OpenFile(lockPath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("mailbox: open lock file: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("mailbox: acquire lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn()
}

// ReadMailbox reads all messages from a teammate's inbox.
// Returns an empty slice (never nil) if the inbox file does not exist.
func ReadMailbox(agentName, teamName, homeDir string) ([]Message, error) {
	inboxPath := getInboxPath(agentName, teamName, homeDir)

	data, err := os.ReadFile(inboxPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Message{}, nil
		}
		return nil, fmt.Errorf("mailbox: read inbox: %w", err)
	}

	var messages []Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("mailbox: parse inbox: %w", err)
	}
	return messages, nil
}

// ReadUnreadMessages reads only unread messages from a teammate's inbox.
func ReadUnreadMessages(agentName, teamName, homeDir string) ([]Message, error) {
	messages, err := ReadMailbox(agentName, teamName, homeDir)
	if err != nil {
		return nil, err
	}

	var unread []Message
	for _, m := range messages {
		if !m.Read {
			unread = append(unread, m)
		}
	}
	return unread, nil
}

// WriteToMailbox writes a message to a teammate's inbox.
// Uses file locking to prevent race conditions when multiple agents write concurrently.
// The message is always appended with Read set to false.
func WriteToMailbox(recipientName string, msg Message, teamName, homeDir string) error {
	if err := ensureInboxDir(recipientName, teamName, homeDir); err != nil {
		return err
	}

	inboxPath := getInboxPath(recipientName, teamName, homeDir)
	lockPath := getLockPath(inboxPath)

	// Ensure the inbox file exists before locking (atomic create-if-not-exists).
	f, err := os.OpenFile(inboxPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		_, _ = f.WriteString("[]")
		f.Close()
	} else if !os.IsExist(err) {
		return fmt.Errorf("mailbox: create inbox file: %w", err)
	}

	globalMu.Lock()
	defer globalMu.Unlock()

	return withLock(lockPath, func() error {
		messages, err := ReadMailbox(recipientName, teamName, homeDir)
		if err != nil {
			return err
		}

		msg.Read = false
		messages = append(messages, msg)

		data, err := json.MarshalIndent(messages, "", "  ")
		if err != nil {
			return fmt.Errorf("mailbox: marshal messages: %w", err)
		}

		return os.WriteFile(inboxPath, data, 0o644)
	})
}

// MarkMessagesAsRead marks all messages in a teammate's inbox as read.
func MarkMessagesAsRead(agentName, teamName, homeDir string) error {
	messages, err := ReadMailbox(agentName, teamName, homeDir)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}

	if err := ensureInboxDir(agentName, teamName, homeDir); err != nil {
		return err
	}

	inboxPath := getInboxPath(agentName, teamName, homeDir)
	lockPath := getLockPath(inboxPath)

	globalMu.Lock()
	defer globalMu.Unlock()

	return withLock(lockPath, func() error {
		// Re-read messages after acquiring lock to get the latest state.
		messages, err := ReadMailbox(agentName, teamName, homeDir)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			return nil
		}

		for i := range messages {
			messages[i].Read = true
		}

		data, err := json.MarshalIndent(messages, "", "  ")
		if err != nil {
			return fmt.Errorf("mailbox: marshal messages: %w", err)
		}

		return os.WriteFile(inboxPath, data, 0o644)
	})
}

// ClearMailbox clears all messages from a teammate's inbox.
// Uses O_RDWR flag so ENOENT is returned if the file doesn't exist, avoiding
// accidental creation of a new inbox.
func ClearMailbox(agentName, teamName, homeDir string) error {
	inboxPath := getInboxPath(agentName, teamName, homeDir)

	f, err := os.OpenFile(inboxPath, os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("mailbox: clear inbox: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString("[]")
	return err
}

// MarkMessageAsReadByIndex marks a single message at the given index as read.
// Uses file locking to prevent race conditions when multiple agents read/write
// concurrently. Returns nil if the index is out of bounds or the message is
// already read.
func MarkMessageAsReadByIndex(agentName, teamName, homeDir string, messageIndex int) error {
	inboxPath := getInboxPath(agentName, teamName, homeDir)
	lockPath := getLockPath(inboxPath)

	globalMu.Lock()
	defer globalMu.Unlock()

	return withLock(lockPath, func() error {
		messages, err := ReadMailbox(agentName, teamName, homeDir)
		if err != nil {
			return err
		}
		if messageIndex < 0 || messageIndex >= len(messages) {
			return nil
		}
		if messages[messageIndex].Read {
			return nil
		}

		messages[messageIndex].Read = true

		data, err := json.MarshalIndent(messages, "", "  ")
		if err != nil {
			return fmt.Errorf("mailbox: marshal messages: %w", err)
		}
		return os.WriteFile(inboxPath, data, 0o644)
	})
}

// MarkMessagesAsReadByPredicate marks all messages matching the predicate as
// read. Uses file locking to prevent race conditions. Unread messages that
// match the predicate are updated; already-read messages and non-matching
// messages are left unchanged.
func MarkMessagesAsReadByPredicate(agentName, teamName, homeDir string, predicate func(Message) bool) error {
	inboxPath := getInboxPath(agentName, teamName, homeDir)
	lockPath := getLockPath(inboxPath)

	globalMu.Lock()
	defer globalMu.Unlock()

	return withLock(lockPath, func() error {
		messages, err := ReadMailbox(agentName, teamName, homeDir)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			return nil
		}

		changed := false
		for i, m := range messages {
			if !m.Read && predicate(m) {
				messages[i].Read = true
				changed = true
			}
		}
		if !changed {
			return nil
		}

		data, err := json.MarshalIndent(messages, "", "  ")
		if err != nil {
			return fmt.Errorf("mailbox: marshal messages: %w", err)
		}
		return os.WriteFile(inboxPath, data, 0o644)
	})
}
