package sandbox

import (
	"os"
	"runtime"
	"strings"
)

// Platform represents the detected operating system platform.
type Platform string

const (
	PlatformMacOS  Platform = "macos"
	PlatformLinux  Platform = "linux"
	PlatformWSL    Platform = "wsl"
	PlatformWSL1   Platform = "wsl1"
	PlatformWSL2   Platform = "wsl2"
	PlatformOther  Platform = "other"
)

// SupportedPlatforms returns the list of platforms where sandboxing can work.
var supportedPlatforms = []Platform{PlatformMacOS, PlatformLinux, PlatformWSL2}

// DetectPlatform returns the current platform identifier.
// For WSL detection, checks /proc/version for "microsoft" or "WSL".
// Distinguishes WSL1 from WSL2 by checking /proc/version for version string.
func DetectPlatform() Platform {
	if runtime.GOOS == "darwin" {
		return PlatformMacOS
	}
	if runtime.GOOS != "linux" {
		return PlatformOther
	}
	versionData, err := os.ReadFile("/proc/version")
	if err != nil {
		return PlatformLinux
	}
	content := strings.ToLower(string(versionData))
	if !strings.Contains(content, "microsoft") && !strings.Contains(content, "wsl") {
		return PlatformLinux
	}
	// WSL2 kernel version contains "WSL2" in /proc/version, WSL1 does not.
	// WSL1 uses a different kernel implementation that does not support
	// user namespaces required by bubblewrap.
	if strings.Contains(content, "wsl2") {
		return PlatformWSL2
	}
	// Check for the WSL2 marker in /proc/sys/kernel/osrelease
	if osRelease, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		if strings.Contains(strings.ToLower(string(osRelease)), "wsl2") {
			return PlatformWSL2
		}
		return PlatformWSL1
	}
	return PlatformWSL1
}

// IsSupportedPlatform reports whether the current platform supports sandboxing.
func IsSupportedPlatform() bool {
	platform := DetectPlatform()
	for _, sp := range supportedPlatforms {
		if platform == sp {
			return true
		}
	}
	return false
}

// IsPlatformInList checks if the current platform is in the given allow list.
// An empty or nil list means all platforms are allowed.
func IsPlatformInList(enabledPlatforms []Platform) bool {
	if len(enabledPlatforms) == 0 {
		return true
	}
	current := DetectPlatform()
	for _, p := range enabledPlatforms {
		if p == current {
			return true
		}
	}
	return false
}

// WSL2DetectionPath is exposed for testing — the path checked after
// /proc/version when distinguishing WSL1 from WSL2.
var WSL2DetectionPath = "/proc/sys/kernel/osrelease"
