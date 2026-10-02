package gatedrive

import (
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// These are the store-level slot run-fence tests (change 0375 Task 9): the
// slot's run fence refuses any later reservation that does not carry the owning
// run. Drive starts no longer reserve the slot (change 0490 moved drive
// admission onto the worktree lock); the slot store and these tests go with it.

// TestRunOmissionCannotDetachOwnedWorktree proves the slot's run fence: a
// slot owned by run E admits only E's own sequential drives. A reservation
// carrying a different run (or none) is refused ErrStaleRunID — while the slot
// is still owned AND after it is released between drives — and only E readmits,
// preserving the run.
func TestRunOmissionCannotDetachOwnedWorktree(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)

	owning := sampleAdmission(wt)
	owning.RunID = "E"
	token, err := s.ReserveWorktreeExecution(owning)
	if err != nil {
		t.Fatalf("reserve for run E: %v", err)
	}

	empty := sampleAdmission(wt)
	empty.RunID = ""
	other := sampleAdmission(wt)
	other.RunID = "E-prime"

	// While the slot is still reserved (owned, mid-flight) the run fence precedes
	// the plain worktree-busy check: an omitted or different run is stale-run-id,
	// not worktree-busy.
	if _, err := s.ReserveWorktreeExecution(empty); !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("owned+omitted run must be ErrStaleRunID, got %v", err)
	}
	if _, err := s.ReserveWorktreeExecution(other); !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("owned+different run must be ErrStaleRunID, got %v", err)
	}

	// Release the slot (the between-drives window). It still belongs to E.
	if err := s.ReleaseWorktreeExecution(wt, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := s.ReserveWorktreeExecution(empty); !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("released+omitted run must be ErrStaleRunID, got %v", err)
	}
	if _, err := s.ReserveWorktreeExecution(other); !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("released+different run must be ErrStaleRunID, got %v", err)
	}

	// The owning run readmits its own next sequential drive, preserving the run.
	if _, err := s.ReserveWorktreeExecution(owning); err != nil {
		t.Fatalf("owning run E must readmit its own drive, got %v", err)
	}
	got, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.RunID != "E" {
		t.Fatalf("readmission must preserve run E, got %q", got.RunID)
	}
}

// TestStandaloneSlotFencesNoRun proves a slot with no run (a standalone gate)
// fences nothing: a released no-run-record slot readmits any reservation, run-carrying
// or not — the fence is not a blanket refusal.
func TestStandaloneSlotFencesNoRun(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)

	plain := sampleAdmission(wt) // RunID: ""
	token, err := s.ReserveWorktreeExecution(plain)
	if err != nil {
		t.Fatalf("reserve standalone: %v", err)
	}
	if err := s.ReleaseWorktreeExecution(wt, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	// A no-run-record slot admits a later run-carrying reservation: it detaches
	// nothing (no run owned it).
	withRun := sampleAdmission(wt)
	withRun.RunID = "E"
	if _, err := s.ReserveWorktreeExecution(withRun); err != nil {
		t.Fatalf("a no-run-record released slot must readmit, got %v", err)
	}
}
