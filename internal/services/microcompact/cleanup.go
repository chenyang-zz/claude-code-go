package microcompact

// PostCompactCleanup runs cleanup after compaction. In Go, this is simplified
// compared to the TS version (postCompactCleanup.ts) — most of the TS cleanup
// targets React/Ink module-level caches and ant-only features that have no
// equivalent in the Go runtime.
//
// Currently only resets the microcompact service's own state (compact warning
// suppression). Additional subsystem cleanup can be wired here as those
// subsystems are ported.
func (s *MicrocompactService) PostCompactCleanup() {
	s.ClearCompactWarningSuppression()
}
