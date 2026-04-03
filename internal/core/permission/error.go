package permission

import "errors"

// ErrUnauthorized reports that a filesystem request was not granted by the current permission policy.
var ErrUnauthorized = errors.New("permission: unauthorized")

// PermissionError is the stable caller-facing error returned for deny/ask permission outcomes.
type PermissionError struct {
	// ToolName identifies the tool whose request was blocked or requires approval.
	ToolName string
	// Path stores the raw user-facing path associated with the failed request.
	Path string
	// Access records whether the request was a read or write check.
	Access Access
	// Decision preserves whether the policy denied the request or requires approval.
	Decision Decision
	// Rule points to the explicit rule responsible for the outcome when one matched.
	Rule *Rule
	// Message stores the stable error message exposed to callers.
	Message string
}

// Error returns the stable caller-facing message for the unauthorized request.
func (e *PermissionError) Error() string {
	return e.Message
}

// Unwrap exposes a sentinel so callers can recognize permission failures with errors.Is.
func (e *PermissionError) Unwrap() error {
	return ErrUnauthorized
}

// ToError converts a non-allow evaluation into a stable permission error for callers.
func (e Evaluation) ToError(req FilesystemRequest) error {
	if e.Decision == DecisionAllow {
		return nil
	}

	return &PermissionError{
		ToolName: req.ToolName,
		Path:     req.Path,
		Access:   req.Access,
		Decision: e.Decision,
		Rule:     e.Rule,
		Message:  e.Message,
	}
}
