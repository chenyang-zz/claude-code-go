package permission

import "context"

type grantedFilesystemAccessKey struct{}
type grantedBashAccessKey struct{}

type grantedFilesystemAccess struct {
	toolName   string
	path       string
	workingDir string
	access     Access
}

type grantedBashAccess struct {
	toolName   string
	command    string
	workingDir string
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

// WithBashGrant attaches one in-memory Bash approval grant to the context for the duration of one retry.
func WithBashGrant(ctx context.Context, req BashRequest) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	grants, _ := ctx.Value(grantedBashAccessKey{}).([]grantedBashAccess)
	next := append([]grantedBashAccess(nil), grants...)
	next = append(next, grantedBashAccess{
		toolName:   req.ToolName,
		command:    req.Command,
		workingDir: req.WorkingDir,
	})
	return context.WithValue(ctx, grantedBashAccessKey{}, next)
}

// HasBashGrant reports whether the current context already carries a matching Bash approval grant.
func HasBashGrant(ctx context.Context, req BashRequest) bool {
	if ctx == nil {
		return false
	}

	grants, _ := ctx.Value(grantedBashAccessKey{}).([]grantedBashAccess)
	for _, grant := range grants {
		if grant.toolName == req.ToolName && grant.command == req.Command && grant.workingDir == req.WorkingDir {
			return true
		}
	}
	return false
}

type grantedWebFetchAccessKey struct{}

type grantedWebFetchAccess struct {
	toolName   string
	url        string
	workingDir string
}

// WithWebFetchGrant attaches one in-memory WebFetch approval grant to the context for the duration of one retry.
func WithWebFetchGrant(ctx context.Context, toolName, rawURL, workingDir string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	grants, _ := ctx.Value(grantedWebFetchAccessKey{}).([]grantedWebFetchAccess)
	next := append([]grantedWebFetchAccess(nil), grants...)
	next = append(next, grantedWebFetchAccess{
		toolName:   toolName,
		url:        rawURL,
		workingDir: workingDir,
	})
	return context.WithValue(ctx, grantedWebFetchAccessKey{}, next)
}

// HasWebFetchGrant reports whether the current context already carries a matching WebFetch approval grant.
func HasWebFetchGrant(ctx context.Context, toolName, rawURL, workingDir string) bool {
	if ctx == nil {
		return false
	}

	grants, _ := ctx.Value(grantedWebFetchAccessKey{}).([]grantedWebFetchAccess)
	for _, grant := range grants {
		if grant.toolName == toolName && grant.url == rawURL && grant.workingDir == workingDir {
			return true
		}
	}
	return false
}
