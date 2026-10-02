// Restart modeling for the gate driver's fault and recovery tests. A
// mid-transition crash is modeled by hand-driving the durable first half of a
// two-phase transition and then STOPPING before the second, which is exactly
// what a fresh Store observes after a real crash; reopenStore gives such a test
// that fresh Store.
//
// The worktree execution slot's fault cases that lived here (change 0405 Task 7,
// change 0375) are gone with the slot: the worktree lock that replaced it
// (change 0490) is released by the kernel when its holder dies, so a crash
// leaves nothing to recover. The lock's own failure paths are pinned in
// driver_test.go (TestLaunchFailureFreesWorktree,
// TestAbandonAdmissionFreesWorktree), in driver_launch_claim_test.go
// (TestStartAdmittedRefusesBusyClaim), and, against the real supervisor, in internal/process and the gatedrive
// integration corpus.
package gatedrive

import "path/filepath"

// reopenStore models a process restart: OpenStore roots a store at
// <gitCommonDir>/docket/gate-drives/v2, so recovering the common dir and opening a
// FRESH store over it exercises the same durable records a restarted process reads,
// with no carried-over in-memory state.
func reopenStore(s *Store) *Store {
	common := filepath.Dir(filepath.Dir(filepath.Dir(s.root)))
	return OpenStore(common)
}
