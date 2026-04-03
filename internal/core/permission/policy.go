package permission

import "context"

type PolicyEngine interface {
	EvaluateTool(ctx context.Context, req ToolRequest) Decision
	EvaluateCommand(ctx context.Context, req CommandRequest) Decision
}
