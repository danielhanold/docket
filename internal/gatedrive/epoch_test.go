package gatedrive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// These are the gatedrive-side run-epoch tests (change 0375 Task 9). A scoped
// start threads its RunEpochID onto the worktree execution slot, and the slot's
// epoch fence refuses any later reservation that does not carry the owning epoch —
// an omitted or stale epoch cannot detach a workflow-owned worktree, even over the
// released (between-drives) slot the epoch still owns. The scope schema's v2->v3
// ride-along tolerates a legacy in-flight scope record.

// TestScopedStartCarriesEpochIntoSlot proves a scoped Start records its RunEpochID
// on the worktree execution slot it reserves, so the fence links the worktree to
// the workflow epoch.
func TestScopedStartCarriesEpochIntoSlot(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	_, req := prepareScopedStart(t, store)
	req.RunEpochID = "epoch-carry-xyz"

	proc := &fakeProc{} // launch + observe running
	d := scopedTestDriver(store, clk, proc, stableGit())

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
	if slot.RunEpochID != "epoch-carry-xyz" {
		t.Fatalf("the slot must record the start's run epoch, got %q", slot.RunEpochID)
	}
	if slot.State != admissionExecuting {
		t.Fatalf("after a launched Start the slot must be executing, got %q", slot.State)
	}
}

// TestEpochOmissionCannotDetachOwnedWorktree proves the slot's run-epoch fence: a
// slot owned by epoch E admits only E's own sequential drives. A reservation
// carrying a different epoch (or none) is refused ErrStaleRunEpoch — while the slot
// is still owned AND after it is released between drives — and only E readmits,
// preserving the epoch.
func TestEpochOmissionCannotDetachOwnedWorktree(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)

	owning := sampleAdmission(wt)
	owning.RunEpochID = "E"
	token, err := s.ReserveWorktreeExecution(owning)
	if err != nil {
		t.Fatalf("reserve for epoch E: %v", err)
	}

	empty := sampleAdmission(wt)
	empty.RunEpochID = ""
	other := sampleAdmission(wt)
	other.RunEpochID = "E-prime"

	// While the slot is still reserved (owned, mid-flight) the epoch fence precedes
	// the plain worktree-busy check: an omitted or different epoch is stale-run-epoch,
	// not worktree-busy.
	if _, err := s.ReserveWorktreeExecution(empty); !isOwnership(err, ErrStaleRunEpoch) {
		t.Fatalf("owned+omitted epoch must be ErrStaleRunEpoch, got %v", err)
	}
	if _, err := s.ReserveWorktreeExecution(other); !isOwnership(err, ErrStaleRunEpoch) {
		t.Fatalf("owned+different epoch must be ErrStaleRunEpoch, got %v", err)
	}

	// Release the slot (the between-drives window). It still belongs to E.
	if err := s.ReleaseWorktreeExecution(wt, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := s.ReserveWorktreeExecution(empty); !isOwnership(err, ErrStaleRunEpoch) {
		t.Fatalf("released+omitted epoch must be ErrStaleRunEpoch, got %v", err)
	}
	if _, err := s.ReserveWorktreeExecution(other); !isOwnership(err, ErrStaleRunEpoch) {
		t.Fatalf("released+different epoch must be ErrStaleRunEpoch, got %v", err)
	}

	// The owning epoch readmits its own next sequential drive, preserving the epoch.
	if _, err := s.ReserveWorktreeExecution(owning); err != nil {
		t.Fatalf("owning epoch E must readmit its own drive, got %v", err)
	}
	got, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.RunEpochID != "E" {
		t.Fatalf("readmission must preserve epoch E, got %q", got.RunEpochID)
	}
}

// TestStandaloneSlotFencesNoEpoch proves a slot with no epoch (a standalone gate)
// fences nothing: a released epoch-less slot readmits any reservation, epoch-carrying
// or not — the fence is not a blanket refusal.
func TestStandaloneSlotFencesNoEpoch(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)

	plain := sampleAdmission(wt) // RunEpochID: ""
	token, err := s.ReserveWorktreeExecution(plain)
	if err != nil {
		t.Fatalf("reserve standalone: %v", err)
	}
	if err := s.ReleaseWorktreeExecution(wt, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	// An epoch-less slot admits a later epoch-carrying reservation: it detaches
	// nothing (no epoch owned it).
	withEpoch := sampleAdmission(wt)
	withEpoch.RunEpochID = "E"
	if _, err := s.ReserveWorktreeExecution(withEpoch); err != nil {
		t.Fatalf("an epoch-less released slot must readmit, got %v", err)
	}
}

// TestScopeSchemaV2LegacyTolerated proves the scope schema v2->v3 ride-along
// tolerates a legacy in-flight v2 scope record: it LOADS with an empty RunEpochID
// (never fails closed), and the next CAS write stamps it forward to v3.
func TestScopeSchemaV2LegacyTolerated(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	g, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	// Hand-write a v2 envelope: the slot-lifecycle shape with NO run_epoch_id.
	v2 := `{"generation":"x","record":{"schema_version":2,"repo_identity":"repo-x","child_cap_hash":"` +
		capHash(g.ChildCapability) + `","parent_cap_hash":"` + capHash(g.ParentCapability) +
		`","current_drive_state":"","drive_count":0,"closed":false}}`
	path := filepath.Join(s.scopeRoot, g.ScopeID, recordFileName)
	if err := os.WriteFile(path, []byte(v2), 0o600); err != nil {
		t.Fatalf("write v2: %v", err)
	}
	// v2 is tolerated: it loads with an empty epoch (never ErrUnknownSchema).
	rec, err := s.LoadScope(g.ScopeID)
	if err != nil {
		t.Fatalf("a v2 scope record must be tolerated, got %v", err)
	}
	if rec.RunEpochID != "" {
		t.Fatalf("a legacy v2 record must read an empty epoch, got %q", rec.RunEpochID)
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
