package permission

type Decision int

const (
	Deny Decision = iota
	Ask
	Allow
)
