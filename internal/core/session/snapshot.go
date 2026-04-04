package session

// Snapshot carries the restored or newly initialized session state for one runtime turn.
type Snapshot struct {
	// Session stores the normalized conversation state itself.
	Session Session
	// Resumed reports whether the snapshot came from previously persisted state.
	Resumed bool
}

// Clone returns a detached snapshot copy.
func (s Snapshot) Clone() Snapshot {
	return Snapshot{
		Session: s.Session.Clone(),
		Resumed: s.Resumed,
	}
}
