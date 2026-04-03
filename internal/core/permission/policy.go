package permission

import "context"

// Policy evaluates normalized filesystem permission requests.
type Policy interface {
	// EvaluateFilesystem returns the permission outcome for one filesystem request.
	EvaluateFilesystem(ctx context.Context, req FilesystemRequest) Evaluation
}
