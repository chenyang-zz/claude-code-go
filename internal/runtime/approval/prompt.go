package approval

// Prompt stores the minimal caller-facing approval text rendered by the CLI surface.
type Prompt struct {
	// Title is the short approval headline shown to the user.
	Title string
	// Body is the detailed explanation for the pending approval request.
	Body string
}
