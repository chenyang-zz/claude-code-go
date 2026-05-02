package tips

// Tip represents a single user-facing hint shown during the spinner wait state.
type Tip struct {
	ID               string
	Content          string
	CooldownSessions int
	IsRelevant       func() bool
}
