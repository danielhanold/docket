package gatedrive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// These are the gatedrive-side run-epoch tests (change 0375 Task 9). A scoped
// start threads its RunID onto the worktree execution slot, and the slot's
// epoch fence refuses any later reservation that does not carry the owning epoch —
// an omitted or stale epoch cannot detach a workflow-owned worktree, even over the
// released (between-drives) slot the epoch still owns. The scope schema's v2->v3
// ride-along tolerates a legacy in-flight scope record.

// TestScopedStartCarriesRunIntoSlot proves a scoped Start records its RunID
// on the worktree execution slot it reserves, so the fence links the worktree to
// the workflow epoch.
func TestScopedStartCarriesRunIntoSlot(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	_, req := prepareScopedStart(t, store)
	req.RunID = "epoch-carry-xyz"

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
	if slot.RunID != "epoch-carry-xyz" {
		t.Fatalf("the slot must record the start's run epoch, got %q", slot.RunID)
	}
	if slot.State != admissionExecuting {
		t.Fatalf("after a launched Start the slot must be executing, got %q", slot.State)
	}
}

// TestRunOmissionCannotDetachOwnedWorktree proves the slot's run-epoch fence: a
// slot owned by epoch E admits only E's own sequential drives. A reservation
// carrying a different epoch (or none) is refused ErrStaleRunID — while the slot
// is still owned AND after it is released between drives — and only E readmits,
// preserving the epoch.
func TestRunOmissionCannotDetachOwnedWorktree(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)

	owning := sampleAdmission(wt)
	owning.RunID = "E"
	token, err := s.ReserveWorktreeExecution(owning)
	if err != nil {
		t.Fatalf("reserve for epoch E: %v", err)
	}

	empty := sampleAdmission(wt)
	empty.RunID = ""
	other := sampleAdmission(wt)
	other.RunID = "E-prime"

	// While the slot is still reserved (owned, mid-flight) the epoch fence precedes
	// the plain worktree-busy check: an omitted or different epoch is stale-run-id,
	// not worktree-busy.
	if _, err := s.ReserveWorktreeExecution(empty); !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("owned+omitted epoch must be ErrStaleRunID, got %v", err)
	}
	if _, err := s.ReserveWorktreeExecution(other); !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("owned+different epoch must be ErrStaleRunID, got %v", err)
	}

	// Release the slot (the between-drives window). It still belongs to E.
	if err := s.ReleaseWorktreeExecution(wt, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := s.ReserveWorktreeExecution(empty); !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("released+omitted epoch must be ErrStaleRunID, got %v", err)
	}
	if _, err := s.ReserveWorktreeExecution(other); !isOwnership(err, ErrStaleRunID) {
		t.Fatalf("released+different epoch must be ErrStaleRunID, got %v", err)
	}

	// The owning epoch readmits its own next sequential drive, preserving the epoch.
	if _, err := s.ReserveWorktreeExecution(owning); err != nil {
		t.Fatalf("owning epoch E must readmit its own drive, got %v", err)
	}
	got, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.RunID != "E" {
		t.Fatalf("readmission must preserve epoch E, got %q", got.RunID)
	}
}

// TestStandaloneSlotFencesNoRun proves a slot with no epoch (a standalone gate)
// fences nothing: a released epoch-less slot readmits any reservation, epoch-carrying
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
	// An epoch-less slot admits a later epoch-carrying reservation: it detaches
	// nothing (no epoch owned it).
	withRun := sampleAdmission(wt)
	withRun.RunID = "E"
	if _, err := s.ReserveWorktreeExecution(withRun); err != nil {
		t.Fatalf("an epoch-less released slot must readmit, got %v", err)
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
	// v2 is tolerated: it loads with an empty epoch (never ErrUnknownSchema).
	rec, err := s.LoadScope(g.ScopeID)
	if err != nil {
		t.Fatalf("a v2 scope record must be tolerated, got %v", err)
	}
	if rec.RunID != "" {
		t.Fatalf("a legacy v2 record must read an empty epoch, got %q", rec.RunID)
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

// ---------------------------------------------------------------------------
// Scoped starts inherit the scope's run epoch (change 0467). A scope prepared
// with epoch E hands E to every scoped start under it: a start presenting no
// epoch is admitted as E (it used to be refused stale-run-id against the
// E-owned slot), presenting E still admits, presenting F != E is refused
// scope-identity-mismatch before anything is reserved, and a scope with no
// epoch leaves the presented value governing, exactly as before.
// ---------------------------------------------------------------------------

// recordingRunLaunchGate is a permissive RunLaunchGate that records every epoch id
// it is asked to validate, so a test can prove which epoch the driver gated on.
type recordingRunLaunchGate struct {
	mu   sync.Mutex
	seen []string
}

func (g *recordingRunLaunchGate) gate() RunLaunchGate {
	return func(runID, _ string, reserve func() error) error {
		g.mu.Lock()
		g.seen = append(g.seen, runID)
		g.mu.Unlock()
		return reserve()
	}
}

func (g *recordingRunLaunchGate) runIDs() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.seen...)
}

// prepareRunScopedStart prepares a scope pinned to scopeRun ("" for a scope
// with no epoch) over the sample worktree and returns a StartRequest wired to it
// that presents NO run epoch.
func prepareRunScopedStart(t *testing.T, store *Store, scopeRun string) StartRequest {
	t.Helper()
	req := sampleStart()
	sreq := scopeReqFor(req, "")
	sreq.RunID = scopeRun
	grant, err := store.PrepareScope(sreq)
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	return req
}

// TestScopedStartInheritsScopeRun: a start that presents no epoch, under a
// scope pinned to epoch-e1, over a released slot epoch-e1 still owns, is
// admitted as epoch-e1 — the slot keeps epoch-e1 and every epoch-gate call the
// start made named epoch-e1 (an empty epoch would bypass the gate entirely).
func TestScopedStartInheritsScopeRun(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	req := prepareRunScopedStart(t, store, "epoch-e1")
	releasedRunSlot(t, store, req.Worktree, req.RepoDir)

	g := &recordingRunLaunchGate{}
	d := scopedTestDriver(store, clk, &fakeProc{}, stableGit())
	d.SetRunLaunchGate(g.gate())

	doc, err := d.Start(req) // req.RunID == ""
	if err != nil {
		t.Fatalf("a scoped start presenting no epoch must inherit the scope's, got %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("first slice must WAIT, got %s (%s)", doc.Outcome, doc.Cause)
	}
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.State != admissionExecuting || slot.RunID != "epoch-e1" {
		t.Fatalf("slot = %s/%q, want executing/epoch-e1", slot.State, slot.RunID)
	}
	seen := g.runIDs()
	if len(seen) == 0 {
		t.Fatal("the start never consulted the epoch gate: it ran epoch-less")
	}
	for _, e := range seen {
		if e != "epoch-e1" {
			t.Fatalf("epoch gate consulted with %q, want only epoch-e1 (all calls: %v)", e, seen)
		}
	}
}

// TestScopedStartPresentingScopeRunAdmits: presenting the scope's own epoch
// still admits (regression pin — green before and after this change).
func TestScopedStartPresentingScopeRunAdmits(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	req := prepareRunScopedStart(t, store, "epoch-e1")
	releasedRunSlot(t, store, req.Worktree, req.RepoDir)
	req.RunID = "epoch-e1"

	d := scopedTestDriver(store, clk, &fakeProc{}, stableGit())
	doc, err := d.Start(req)
	if err != nil || doc.Outcome != WAITING {
		t.Fatalf("presenting the scope's epoch must admit and WAIT: doc=%+v err=%v", doc, err)
	}
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.RunID != "epoch-e1" {
		t.Fatalf("slot epoch = %q, want epoch-e1", slot.RunID)
	}
}

// TestScopedStartForeignRunRefused: presenting an epoch that differs from the
// scope's pinned one is refused scope-identity-mismatch before anything is
// reserved — the worktree slot is byte-for-byte untouched, the scope's single
// slot stays empty, nothing launched, and the epoch gate was never consulted.
func TestScopedStartForeignRunRefused(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	req := prepareRunScopedStart(t, store, "epoch-e1")
	releasedRunSlot(t, store, req.Worktree, req.RepoDir)
	req.RunID = "epoch-foreign"
	before := readSlotBytes(t, store, req.Worktree)

	g := &recordingRunLaunchGate{}
	proc := &fakeProc{}
	d := scopedTestDriver(store, clk, proc, stableGit())
	d.SetRunLaunchGate(g.gate())

	if _, err := d.Start(req); !isOwnershipKind(err, ErrScopeIdentityMismatch) {
		t.Fatalf("a foreign presented epoch must refuse scope-identity-mismatch, got %v", err)
	}
	if string(readSlotBytes(t, store, req.Worktree)) != string(before) {
		t.Fatal("a refused start must not touch the worktree slot")
	}
	scope, err := store.LoadScope(req.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.CurrentDriveID != "" || scope.DriveCount != 0 {
		t.Fatalf("a refused start must reserve no scope slot, got current=%q count=%d", scope.CurrentDriveID, scope.DriveCount)
	}
	if proc.launchN != 0 {
		t.Fatalf("a refused start must launch nothing, launched %d", proc.launchN)
	}
	if n := len(g.runIDs()); n != 0 {
		t.Fatalf("a refused start must not reach the epoch gate, consulted %d times", n)
	}
}

// TestScopedSuccessorStartInheritsScopeRun: the worker's SEQUENCE of drives in
// one scope — a successor start presenting no epoch after a PASSED predecessor
// over the still-executing, epoch-e1-owned slot — is admitted (it rotates the
// slot), while a successor presenting a foreign epoch is refused
// scope-identity-mismatch without consuming the predecessor receipt.
func TestScopedSuccessorStartInheritsScopeRun(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	req := prepareRunScopedStart(t, store, "epoch-e1")
	d := scopedTestDriver(store, clk, &fakeProc{}, stableGit())

	first, err := d.Start(req) // presents no epoch: inherits epoch-e1
	if err != nil || first.Outcome != WAITING {
		t.Fatalf("first start must inherit and WAIT: doc=%+v err=%v", first, err)
	}
	if err := store.ownerCAS(first.DriveID, func(r *driveRecord) error {
		r.LastOutcome = PASSED
		return nil
	}); err != nil {
		t.Fatalf("settle predecessor terminal: %v", err)
	}

	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation

	foreign := succ
	foreign.RunID = "epoch-foreign"
	if _, err := d.Start(foreign); !isOwnershipKind(err, ErrScopeIdentityMismatch) {
		t.Fatalf("a successor presenting a foreign epoch must refuse scope-identity-mismatch, got %v", err)
	}

	second, err := d.Start(succ) // presents no epoch: inherits epoch-e1
	if err != nil {
		t.Fatalf("a successor presenting no epoch must inherit the scope's, got %v", err)
	}
	if second.Outcome != WAITING || second.DriveID == first.DriveID {
		t.Fatalf("successor must be a NEW waiting drive, got %+v", second)
	}
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.RunID != "epoch-e1" {
		t.Fatalf("successor slot epoch = %q, want epoch-e1", slot.RunID)
	}
}

// TestNoRunRecordScopeKeepsPresentedRun: a scope with no pinned epoch (a legacy
// v2 scope, or one prepared without) supplies nothing — the presented value
// governs, unchanged from before. Presenting none over an epoch-e1-owned slot is
// still fenced stale-run-id; presenting epoch-e1 admits.
func TestNoRunRecordScopeKeepsPresentedRun(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	req := prepareRunScopedStart(t, store, "")
	releasedRunSlot(t, store, req.Worktree, req.RepoDir)
	d := scopedTestDriver(store, clk, &fakeProc{}, stableGit())

	if _, err := d.Start(req); !isOwnershipKind(err, ErrStaleRunID) {
		t.Fatalf("an epoch-less scope must not supply an epoch: want stale-run-id, got %v", err)
	}
	req.RunID = "epoch-e1"
	doc, err := d.Start(req)
	if err != nil || doc.Outcome != WAITING {
		t.Fatalf("presenting the owning epoch under an epoch-less scope must admit: doc=%+v err=%v", doc, err)
	}
}

// TestAdvisoryRunIDMatchesAdmit pins the read-only resolution the application
// layer's advisory pre-admission check reconciles with (change 0467): a
// credentialed scoped start presenting no epoch (or the scope's) resolves to the
// scope's pinned epoch; a foreign presented epoch, a rejected capability, an
// unknown scope, a scopeless start, and an epoch-less scope all keep the
// presented value.
func TestAdvisoryRunIDMatchesAdmit(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	d := scopedTestDriver(store, clk, &fakeProc{}, stableGit())
	pinned := prepareRunScopedStart(t, store, "epoch-e1")

	cases := []struct {
		name string
		mut  func(r StartRequest) StartRequest
		want string
	}{
		{"none presented inherits", func(r StartRequest) StartRequest { return r }, "epoch-e1"},
		{"same presented", func(r StartRequest) StartRequest { r.RunID = "epoch-e1"; return r }, "epoch-e1"},
		{"foreign presented kept", func(r StartRequest) StartRequest { r.RunID = "epoch-x"; return r }, "epoch-x"},
		{"bad capability", func(r StartRequest) StartRequest { r.ChildCapability = "nope"; return r }, ""},
		{"unknown scope", func(r StartRequest) StartRequest { r.ScopeID = "00000000000000000000000000000000"; return r }, ""},
		{"scopeless", func(r StartRequest) StartRequest { r.ScopeID = ""; r.RunID = "epoch-s"; return r }, "epoch-s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := d.AdvisoryRunID(tc.mut(pinned)); got != tc.want {
				t.Fatalf("AdvisoryRunID = %q, want %q", got, tc.want)
			}
		})
	}

	store2 := OpenStore(testsupport.TempDir(t))
	d2 := scopedTestDriver(store2, clk, &fakeProc{}, stableGit())
	noRunRecord := prepareRunScopedStart(t, store2, "")
	noRunRecord.RunID = "epoch-p"
	if got := d2.AdvisoryRunID(noRunRecord); got != "epoch-p" {
		t.Fatalf("an epoch-less scope must keep the presented epoch, got %q", got)
	}
}
