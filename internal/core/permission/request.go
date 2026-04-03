package permission

type ToolRequest struct {
	Name     string
	ReadOnly bool
	Input    map[string]any
}

type CommandRequest struct {
	Name string
	Args []string
}
