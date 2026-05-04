package internallogging

import (
	"os"
	"strings"
	"sync"
)

// namespaceFilePath is the well-known Kubernetes ServiceAccount namespace
// path. It is exposed as a package-level var (rather than a const) so tests
// can swap it for a fake path before invoking GetKubernetesNamespace.
var namespaceFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

const namespaceNotFound = "namespace not found"

var (
	namespaceOnce   sync.Once
	cachedNamespace string
)

// GetKubernetesNamespace reads the current Kubernetes namespace from the
// service-account file. The result is memoized for the lifetime of the
// process (matching the TS lodash memoize semantics, which cache failures
// permanently as well).
//
// Return values:
//
//   - ""                       — FlagInternalLogging is disabled (TS null).
//   - "namespace not found"    — file read failed (macOS / non-K8s env).
//   - <trimmed file contents>  — successful read.
func GetKubernetesNamespace() string {
	if !IsInternalLoggingEnabled() {
		return ""
	}
	namespaceOnce.Do(func() {
		cachedNamespace = readNamespaceFile()
	})
	return cachedNamespace
}

// readNamespaceFile is the unmemoized file-read helper. It is non-exported
// so tests exercise the memoized public surface; tests that need a fresh
// read may invoke it directly within the package.
func readNamespaceFile() string {
	data, err := os.ReadFile(namespaceFilePath)
	if err != nil {
		return namespaceNotFound
	}
	return strings.TrimSpace(string(data))
}
