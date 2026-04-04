package permission

import "context"

type grantedFilesystemAccessKey struct{}

type grantedFilesystemAccess struct {
	toolName   string
	path       string
	workingDir string
	access     Access
}

// WithFilesystemGrant attaches one in-memory approval grant to the context for the duration of one retry.
func WithFilesystemGrant(ctx context.Context, req FilesystemRequest) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	grants, _ := ctx.Value(grantedFilesystemAccessKey{}).([]grantedFilesystemAccess)
	next := append([]grantedFilesystemAccess(nil), grants...)
	next = append(next, grantedFilesystemAccess{
		toolName:   req.ToolName,
		path:       req.Path,
		workingDir: req.WorkingDir,
		access:     req.Access,
	})
	return context.WithValue(ctx, grantedFilesystemAccessKey{}, next)
}

// hasFilesystemGrant reports whether the current context already carries a matching approval grant.
func hasFilesystemGrant(ctx context.Context, req FilesystemRequest) bool {
	if ctx == nil {
		return false
	}

	grants, _ := ctx.Value(grantedFilesystemAccessKey{}).([]grantedFilesystemAccess)
	for _, grant := range grants {
		if grant.toolName == req.ToolName && grant.path == req.Path && grant.workingDir == req.WorkingDir && grant.access == req.Access {
			return true
		}
	}
	return false
}
