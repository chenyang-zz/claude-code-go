package fs

import (
	"os"
	"testing"
)

// TestCWDResolverGetCwdPrefersOverride verifies call-scoped working directories win over the fallback.
func TestCWDResolverGetCwdPrefersOverride(t *testing.T) {
	resolver := NewCWDResolverWithOriginal("/workspace/original")

	if got := resolver.GetCwd("/workspace/agent"); got != "/workspace/agent" {
		t.Fatalf("GetCwd() = %q, want %q", got, "/workspace/agent")
	}
}

// TestCWDResolverGetCwdFallsBackToOriginal verifies empty call context reuses the bootstrap directory.
func TestCWDResolverGetCwdFallsBackToOriginal(t *testing.T) {
	resolver := NewCWDResolverWithOriginal("/workspace/original")

	if got := resolver.GetCwd(""); got != "/workspace/original" {
		t.Fatalf("GetCwd() = %q, want %q", got, "/workspace/original")
	}
}

// TestNewCWDResolverCapturesProcessDirectory verifies the resolver snapshots the current process cwd.
func TestNewCWDResolverCapturesProcessDirectory(t *testing.T) {
	want, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	resolver, err := NewCWDResolver()
	if err != nil {
		t.Fatalf("NewCWDResolver() error = %v", err)
	}

	if got := resolver.Original(); got != want {
		t.Fatalf("Original() = %q, want %q", got, want)
	}
}
