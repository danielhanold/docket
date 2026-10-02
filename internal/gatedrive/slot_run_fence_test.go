package gatedrive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// These are the gatedrive-side run tests (change 0375 Task 9). A start
// threads its RunID onto the worktree execution slot, and the slot's
// run fence refuses any later reservation that does not carry the owning run —
// an omitted or stale run cannot detach a workflow-owned worktree, even over the
// released (between-drives) slot the run still owns. The scope schema's v2->v3
// ride-along tolerates a legacy in-flight scope record.

// TestStartCarriesRunIntoSlot proves a Start records its RunID on the worktree
// execution slot it reserves, so the fence links the worktree to the workflow
// run.
func TestStartCarriesRunIntoSlot(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	req := sampleStart()
	req.RunID = "run-carry-xyz"

	proc := &fakeProc{} // launch + observe running
	d := storeTestDriver(store, clk, proc, stableGit())

	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("first slice must WAIT, got %s (%s)", doc.Outcome, doc.Cause)
	}
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.RunID != "run-carry-xyz" {
		t.Fatalf("the slot must record the start's run, got %q", slot.RunID)
	}
	if slot.State != admissionExecuting {
		t.Fatalf("after a launched Start the slot must be executing, got %q", slot.State)
	}
}

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

// TestScopeSchemaV2LegacyTolerated proves the scope schema v2->v3 ride-along
// tolerates a legacy in-flight v2 scope record: it LOADS with an empty RunID
// (never fails closed), and the next CAS write stamps it forward to v3.
func TestScopeSchemaV2LegacyTolerated(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	g, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	// Hand-write a v2 envelope: the slot-lifecycle shape with NO run_id.
	v2 := `{"generation":"x","record":{"schema_version":2,"repo_identity":"repo-x","child_cap_hash":"` +
		capHash(g.ChildCapability) + `","parent_cap_hash":"` + capHash(g.ParentCapability) +
		`","current_drive_state":"","drive_count":0,"closed":false}}`
	path := filepath.Join(s.scopeRoot, g.ScopeID, recordFileName)
	if err := os.WriteFile(path, []byte(v2), 0o600); err != nil {
		t.Fatalf("write v2: %v", err)
	}
	// v2 is tolerated: it loads with an empty run (never ErrUnknownSchema).
	rec, err := s.LoadScope(g.ScopeID)
	if err != nil {
		t.Fatalf("a v2 scope record must be tolerated, got %v", err)
	}
	if rec.RunID != "" {
		t.Fatalf("a legacy v2 record must read an empty run, got %q", rec.RunID)
	}
	// The next CAS write stamps it forward to v3.
	if err := s.closeScope(g.ScopeID); err != nil {
		t.Fatalf("closeScope: %v", err)
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var stored storedScope
	if err := json.Unmarshal(buf, &stored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if stored.Record.SchemaVersion != scopeSchemaVersion {
		t.Fatalf("a CAS write must stamp the record forward to v%d, got v%d", scopeSchemaVersion, stored.Record.SchemaVersion)
	}
}
