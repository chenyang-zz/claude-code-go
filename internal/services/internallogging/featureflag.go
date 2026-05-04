// Package internallogging provides Ant-internal diagnostic logging for the
// permission context. It is the Go port of src/services/internalLogging.ts.
//
// The package is intentionally OS-platform-coupled: it reads
// /var/run/secrets/kubernetes.io/serviceaccount/namespace and
// /proc/self/mountinfo (Linux-only paths) to emit Kubernetes namespace and
// container ID alongside the permission context.
//
// The package is gated on featureflag.FlagInternalLogging
// (CLAUDE_FEATURE_INTERNAL_LOGGING=1). When the flag is disabled, all public
// entry points return immediately and the lookup helpers return the empty
// string (the Go equivalent of the TS null sentinel).
package internallogging

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsInternalLoggingEnabled reports whether the Ant-internal logging path is
// active. This is the Go-side replacement for the TS check
// `process.env.USER_TYPE === 'ant'`, which on the TS side was an esbuild
// build-time --define and not a runtime env-var check.
func IsInternalLoggingEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagInternalLogging)
}
