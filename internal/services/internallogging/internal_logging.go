package internallogging

import (
	"context"
	"encoding/json"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Moment describes when LogPermissionContextForAnts is invoked.
type Moment string

const (
	// MomentInitialization is recorded once during REPL bootstrap.
	MomentInitialization Moment = "initialization"
	// MomentSummary is recorded during the /compact summary flow.
	MomentSummary Moment = "summary"
)

// eventName mirrors the analytics event name emitted by the TS source so
// downstream log scrapers continue to recognise it.
const eventName = "tengu_internal_record_permission_context"

// component is the logger component label for this package.
const component = "internallogging"

// LogPermissionContextForAnts emits a diagnostic log entry capturing the
// current permission context plus the Kubernetes namespace and OCI
// container ID of the host. It is a no-op when FlagInternalLogging is
// disabled, matching the TS guard `process.env.USER_TYPE !== 'ant'`.
//
// The analytics subsystem has not yet been ported, so this implementation
// routes the event through pkg/logger at debug level. Once the analytics
// pipeline lands, callers can swap the body for an analytics dispatch
// without changing the call sites.
//
// toolPermissionContext is intentionally typed as interface{} because the
// permission-context model has not been migrated yet (out of scope for
// batch-248). Callers pass nil during initialization and a concrete value
// during summary generation.
func LogPermissionContextForAnts(
	ctx context.Context,
	toolPermissionContext interface{},
	moment Moment,
) {
	_ = ctx // reserved for future tracing / cancellation hooks

	if !IsInternalLoggingEnabled() {
		return
	}

	namespace := GetKubernetesNamespace()
	containerID := GetContainerID()

	var ctxJSON string
	if toolPermissionContext != nil {
		if b, err := json.Marshal(toolPermissionContext); err == nil {
			ctxJSON = string(b)
		}
	}

	logger.DebugCF(component, eventName, map[string]any{
		"moment":                string(moment),
		"namespace":             namespace,
		"containerID":           containerID,
		"toolPermissionContext": ctxJSON,
	})
}
