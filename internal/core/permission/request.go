package permission

import "fmt"

// Access describes the filesystem operation category being evaluated.
type Access string

const (
	// AccessRead covers non-mutating file reads and search-style traversal.
	AccessRead Access = "read"
	// AccessWrite covers file creation, overwrite, rename, delete, and edit operations.
	AccessWrite Access = "write"
)

// Valid reports whether the access kind is supported by the minimal permission model.
func (a Access) Valid() bool {
	switch a {
	case AccessRead, AccessWrite:
		return true
	default:
		return false
	}
}

// FilesystemRequest is the normalized permission input used by batch-01 file tools.
type FilesystemRequest struct {
	// ToolName identifies the tool issuing the request.
	ToolName string
	// Path stores the raw user-targeted path that will be normalized before matching.
	Path string
	// WorkingDir stores the tool invocation working directory for relative path expansion.
	WorkingDir string
	// Access distinguishes read-style checks from write-style checks.
	Access Access
}

// Validate checks whether the request contains the minimum information needed by the permission layer.
func (r FilesystemRequest) Validate() error {
	if r.ToolName == "" {
		return fmt.Errorf("permission: tool name is required")
	}
	if r.Path == "" {
		return fmt.Errorf("permission: path is required")
	}
	if !r.Access.Valid() {
		return fmt.Errorf("permission: unsupported filesystem access %q", r.Access)
	}
	return nil
}
