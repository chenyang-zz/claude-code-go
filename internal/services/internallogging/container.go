package internallogging

import (
	"os"
	"regexp"
	"strings"
	"sync"
)

// mountinfoFilePath is the per-process mountinfo file used to recover the
// container ID. It is a package-level var so tests can substitute a fixture
// path.
var mountinfoFilePath = "/proc/self/mountinfo"

const (
	containerIDNotFound            = "container ID not found"
	containerIDNotFoundInMountinfo = "container ID not found in mountinfo"
)

// containerIDPattern matches both Docker (/docker/containers/<id>) and
// containerd / CRI-O (/sandboxes/<id>) container IDs. Only 64-char hex IDs
// are accepted.
var containerIDPattern = regexp.MustCompile(`(?:/docker/containers/|/sandboxes/)([0-9a-f]{64})`)

var (
	containerIDOnce   sync.Once
	cachedContainerID string
)

// GetContainerID reads /proc/self/mountinfo and extracts the OCI container
// ID. The result is memoized; subsequent calls return the cached value
// (including failure sentinels) without re-reading the file.
//
// Return values:
//
//   - ""                                       — FlagInternalLogging disabled.
//   - "container ID not found"                 — mountinfo read failed.
//   - "container ID not found in mountinfo"    — file read OK but no match.
//   - <64-char hex>                            — container ID extracted.
func GetContainerID() string {
	if !IsInternalLoggingEnabled() {
		return ""
	}
	containerIDOnce.Do(func() {
		cachedContainerID = readMountinfoFile()
	})
	return cachedContainerID
}

// readMountinfoFile is the unmemoized parse helper. Non-exported so tests
// exercise the memoized public surface.
func readMountinfoFile() string {
	data, err := os.ReadFile(mountinfoFilePath)
	if err != nil {
		return containerIDNotFound
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		match := containerIDPattern.FindStringSubmatch(line)
		if len(match) >= 2 {
			return match[1]
		}
	}
	return containerIDNotFoundInMountinfo
}
