// Event-authorized parent takeover and continuation-lookup tests. These exercise
// the takeover.go transfer (parent capability, single-use scope claim, child
// owner supersession) under the outer recovery scope run.start prepares, and the
// two facade-only read surfaces (FindScopeDriveIDs, ContinuationHandle).
// They reuse the deterministic fake clock/proc/git seams from driver_test.go, so
// no test launches a real process or sleeps for a production duration.
package gatedrive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// outerScopeReqFor builds the outer recovery scope run.start prepares for a
// dispatch whose drives carry req's identity: repo, change, branch, and worktree —
// never a task, phase, run id, or run context (change 0489).
func outerScopeReqFor(req StartRequest) ScopeRequest {
	return ScopeRequest{RepoIdentity: req.RepoDir, ChangeID: req.ChangeID, Branch: req.Branch, Worktree: req.Worktree}
}

// startUnderOuterScope prepares an outer scope matching sampleStart's identity and
// starts a drive whose RunContext is that scope's child capability — exactly how
// run.start's run context links implement-next's suite gates to the run — and
// asserts the first slice WAITs.
func startUnderOuterScope(t *testing.T, d *Driver, store *Store) (ScopeGrant, DriveDoc) {
	t.Helper()
	req := sampleStart()
	grant, err := store.PrepareScope(outerScopeReqFor(req))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.RunContext = grant.ChildCapability
	started, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.Outcome != WAITING {
		t.Fatalf("a drive under the outer scope must WAIT on its first slice, got %s (%s)", started.Outcome, started.Cause)
	}
	return grant, started
}

// overwriteDriveRecord replaces a drive's on-disk record with rec, preserving a
// loadable envelope. Tests use it to inject a scope/drive identity mismatch, a
// past deadline, or a corrupt schema without going through a state transition.
func overwriteDriveRecord(t *testing.T, store *Store, id string, rec driveRecord) {
	t.Helper()
	buf, err := json.Marshal(storedRecord{Generation: "overwrite-gen", Record: rec})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.root, id, recordFileName), buf, 0o600); err != nil {
		t.Fatalf("overwrite drive record: %v", err)
	}
}

func mustReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

// TestTakeoverInvalidatesChildAndMintsParentOwner proves the happy path: a
// takeover of a WAITING outer-scope drive returns a NEW owner generation, the old
// child owner can no longer advance, the new owner advances normally, the scope
// is closed, and the takeover itself neither launched nor stopped any process.
func TestTakeoverInvalidatesChildAndMintsParentOwner(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{} // stays running across slices
	d, store := newTestDriver(t, clk, proc, stableGit())

	grant, started := startUnderOuterScope(t, d, store)

	launchesBefore, stopsBefore := proc.launchN, proc.stopN
	took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, started.DriveID)
	if err != nil {
		t.Fatalf("Takeover: %v", err)
	}
	if took.Outcome == HALTED {
		t.Fatalf("a valid takeover must not HALT: %s", took.Cause)
	}
	if took.Generation == "" || took.Generation == started.Generation {
		t.Fatalf("Takeover must mint a fresh owner generation distinct from the child's, got %q", took.Generation)
	}
	if proc.launchN != launchesBefore || proc.stopN != stopsBefore {
		t.Fatalf("Takeover must not launch or stop any process: launch %d->%d stop %d->%d",
			launchesBefore, proc.launchN, stopsBefore, proc.stopN)
	}

	// The old child owner is superseded: it can no longer advance.
	stale, err := d.Advance(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("stale Advance: %v", err)
	}
	if stale.Outcome != HALTED {
		t.Fatalf("the superseded child owner must HALT, got %s", stale.Outcome)
	}

	// The fresh parent owner advances the same live run normally.
	adv, err := d.Advance(started.DriveID, took.Generation)
	if err != nil {
		t.Fatalf("fresh-owner Advance: %v", err)
	}
	if adv.Outcome != WAITING {
		t.Fatalf("the fresh owner must drive the same live run, got %s (%s)", adv.Outcome, adv.Cause)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("no path here may relaunch the suite, launched %d", proc.launchN)
	}

	// The scope is closed after a takeover.
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if !scope.Closed {
		t.Fatalf("a takeover must close the scope")
	}
}

// TestTakeoverAcceptsDriveThatFinishedAfterScan: the outer scan saw the drive
// live, then the suite finished before the takeover: Takeover, given the drive id
// explicitly, still accepts it and hands the recorded verdict over unchanged — the
// suite that completed in that window is not lost (Review Focus 5). The fresh
// owner's Advance returns the recorded PASSED with the same attempt and no
// relaunch.
func TestTakeoverAcceptsDriveThatFinishedAfterScan(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	running := true
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if running {
				return obs(process.StateRunning, runDir), nil
			}
			return obs(process.StatePassed, runDir), nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	grant, started := startUnderOuterScope(t, d, store)

	// Drive it to a terminal PASSED; Advance leaves the owner generation set.
	running = false
	passed, err := d.Advance(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Advance to terminal: %v", err)
	}
	if passed.Outcome != PASSED {
		t.Fatalf("want PASSED terminal, got %s (%s)", passed.Outcome, passed.Cause)
	}

	launchesBefore := proc.launchN
	took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, started.DriveID)
	if err != nil {
		t.Fatalf("Takeover: %v", err)
	}
	if took.Outcome != PASSED {
		t.Fatalf("takeover of a terminal-unconsumed drive must report PASSED, got %s (%s)", took.Outcome, took.Cause)
	}
	if took.Generation == "" || took.Generation == started.Generation {
		t.Fatalf("takeover must mint a fresh owner, got %q", took.Generation)
	}

	adv, err := d.Advance(started.DriveID, took.Generation)
	if err != nil {
		t.Fatalf("fresh-owner Advance: %v", err)
	}
	if adv.Outcome != PASSED || adv.Attempt != passed.Attempt {
		t.Fatalf("the recorded verdict must be returned unchanged: got %s attempt %d", adv.Outcome, adv.Attempt)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("consuming a terminal must not relaunch, launched %d", proc.launchN)
	}
}

// TestTakeoverFailClosedTable proves every takeover rejection HALTs with its
// distinct cause and mutates NEITHER the drive record nor the scope record.
func TestTakeoverFailClosedTable(t *testing.T) {
	const bogusCap = "ffffffffffffffffffffffffffffffff"

	cases := []struct {
		name  string
		setup func(t *testing.T, d *Driver, store *Store, git *fakeGit) (scopeID, parentCap, driveID string)
		want  string
	}{
		{
			name: "wrong parent capability",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := startUnderOuterScope(t, d, store)
				return grant.ScopeID, bogusCap, started.DriveID
			},
			want: string(ErrScopeCapabilityMismatch),
		},
		{
			name: "child capability presented as parent",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := startUnderOuterScope(t, d, store)
				return grant.ScopeID, grant.ChildCapability, started.DriveID
			},
			want: string(ErrScopeCapabilityMismatch),
		},
		{
			name: "closed scope",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := startUnderOuterScope(t, d, store)
				if err := store.claimScopeForTakeover(grant.ScopeID); err != nil {
					t.Fatalf("claimScopeForTakeover: %v", err)
				}
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: string(ErrScopeClosed),
		},
		{
			name: "identity mismatch branch",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := startUnderOuterScope(t, d, store)
				rec, _ := store.Load(started.DriveID)
				rec.Branch = "feat/other"
				overwriteDriveRecord(t, store, started.DriveID, rec)
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: string(ErrScopeIdentityMismatch),
		},
		{
			name: "identity mismatch worktree",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := startUnderOuterScope(t, d, store)
				rec, _ := store.Load(started.DriveID)
				rec.WorktreePath = "/repo/other"
				overwriteDriveRecord(t, store, started.DriveID, rec)
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: string(ErrScopeIdentityMismatch),
		},
		{
			name: "identity mismatch change",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := startUnderOuterScope(t, d, store)
				rec, _ := store.Load(started.DriveID)
				rec.ChangeID = "9999"
				overwriteDriveRecord(t, store, started.DriveID, rec)
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: string(ErrScopeIdentityMismatch),
		},
		{
			name: "fingerprint drift",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := startUnderOuterScope(t, d, store)
				git.status = "DRIFTED" // the worktree changed since drive start
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: string(ErrFingerprintMismatch),
		},
		{
			name: "expired deadline on a live drive",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := startUnderOuterScope(t, d, store)
				rec, _ := store.Load(started.DriveID)
				rec.Deadline = startRun().Add(-time.Minute) // past, drive still WAITING
				overwriteDriveRecord(t, store, started.DriveID, rec)
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: CauseDeadlineExpired,
		},
		{
			name: "outstanding unclaimed handoff must be claimed",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := startUnderOuterScope(t, d, store)
				if _, err := d.Handoff(started.DriveID, started.Generation); err != nil {
					t.Fatalf("Handoff: %v", err)
				}
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: string(ErrHandoffOutstanding),
		},
		{
			name: "unknown drive schema",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := startUnderOuterScope(t, d, store)
				rec, _ := store.Load(started.DriveID)
				rec.SchemaVersion = driveSchemaVersion + 999
				overwriteDriveRecord(t, store, started.DriveID, rec)
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: CauseSchemaMismatch,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clk := &fakeClock{now: startRun()}
			proc := &fakeProc{}
			git := stableGit()
			d, store := newTestDriver(t, clk, proc, git)

			scopeID, parentCap, driveID := tc.setup(t, d, store, git)

			scopePath := filepath.Join(store.scopeRoot, scopeID, recordFileName)
			scopeBefore := mustReadBytes(t, scopePath)
			var drivePath string
			var driveBefore []byte
			if driveID != "" {
				drivePath = filepath.Join(store.root, driveID, recordFileName)
				driveBefore = mustReadBytes(t, drivePath)
			}
			launchesBefore, stopsBefore := proc.launchN, proc.stopN

			doc, err := d.Takeover(scopeID, parentCap, driveID)
			if err != nil {
				t.Fatalf("Takeover returned a command error, want a HALTED document: %v", err)
			}
			if doc.Outcome != HALTED {
				t.Fatalf("a rejected takeover must HALT, got %s (%s)", doc.Outcome, doc.Cause)
			}
			if !strings.Contains(doc.Cause, tc.want) {
				t.Fatalf("cause must name %q, got %q", tc.want, doc.Cause)
			}
			if proc.launchN != launchesBefore || proc.stopN != stopsBefore {
				t.Fatalf("a rejected takeover must not launch or stop a process: launch %d->%d stop %d->%d",
					launchesBefore, proc.launchN, stopsBefore, proc.stopN)
			}
			if got := mustReadBytes(t, scopePath); string(got) != string(scopeBefore) {
				t.Fatalf("a rejected takeover must not mutate the scope record")
			}
			if driveID != "" {
				if got := mustReadBytes(t, drivePath); string(got) != string(driveBefore) {
					t.Fatalf("a rejected takeover must not mutate the drive record")
				}
			}
		})
	}
}

// TestTakeoverRace proves two goroutines racing Takeover with the same parent
// capability yield exactly one fresh owner; the loser HALTs and the old child
// owner is invalid either way. Run under -race.
func TestTakeoverRace(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	grant, started := startUnderOuterScope(t, d, store)

	var wg sync.WaitGroup
	docs := make([]DriveDoc, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			docs[idx], errs[idx] = d.Takeover(grant.ScopeID, grant.ParentCapability, started.DriveID)
		}(i)
	}
	wg.Wait()

	winners := 0
	for i := range docs {
		if errs[i] != nil {
			t.Fatalf("Takeover returned a command error: %v", errs[i])
		}
		if docs[i].Outcome != HALTED {
			winners++
			if docs[i].Generation == "" || docs[i].Generation == started.Generation {
				t.Fatalf("the winner must mint a fresh owner, got %q", docs[i].Generation)
			}
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one takeover must win, got %d", winners)
	}

	// The old child owner is invalid regardless of which goroutine won.
	stale, err := d.Advance(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("stale Advance: %v", err)
	}
	if stale.Outcome != HALTED {
		t.Fatalf("the superseded child owner must HALT after a race, got %s", stale.Outcome)
	}
}

// TestTakeoverRequiresExplicitDriveID (change 0489): Takeover no longer resolves a
// drive itself; its only caller passes the id the outer scan found. An empty id is
// a command error that mutates neither record.
func TestTakeoverRequiresExplicitDriveID(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	d, store := newTestDriver(t, clk, &fakeProc{}, stableGit())
	grant, _ := startUnderOuterScope(t, d, store)
	scopePath := filepath.Join(store.scopeRoot, grant.ScopeID, recordFileName)
	before := mustReadBytes(t, scopePath)
	if _, err := d.Takeover(grant.ScopeID, grant.ParentCapability, ""); err == nil {
		t.Fatalf("an empty drive id must be a command error")
	}
	if got := mustReadBytes(t, scopePath); string(got) != string(before) {
		t.Fatalf("a refused takeover must not mutate the scope record")
	}
}

// TestFindScopeDriveIDs proves the outer-scope candidate resolver: it lists drives
// matching change + run-context hash that are still live; excludes every finished
// drive (owned or consumed), a wrong run context, a wrong change, and unreadable
// records.
func TestFindScopeDriveIDs(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))

	// An empty store has no drive root at all: no candidates, no error.
	if ids, err := store.FindScopeDriveIDs("0342", capHash("ctx")); err != nil || len(ids) != 0 {
		t.Fatalf("empty store must return no candidates: ids=%v err=%v", ids, err)
	}

	seed := func(change, gateHash string, outcome Outcome, owner string) string {
		t.Helper()
		rec := seedRecord(t)
		rec.ChangeID = change
		rec.RunContextHash = gateHash
		rec.LastOutcome = outcome
		rec.OwnerGeneration = owner
		id, _, err := store.NewDrive(rec)
		if err != nil {
			t.Fatalf("NewDrive: %v", err)
		}
		return id
	}
	h := capHash("ctx")
	other := capHash("other-ctx")

	waiting := seed("0342", h, WAITING, "own-w")
	finishedOwned := seed("0342", h, PASSED, "own-t") // finished, owner still set → excluded
	seed("0342", h, PASSED, "")                       // finished AND consumed → excluded
	seed("0342", other, WAITING, "own-o")             // wrong run context → excluded
	seed("0400", h, WAITING, "own-c")                 // wrong change → excluded

	// A corrupt record must be skipped, never fail the scan.
	corrupt := seed("0342", h, WAITING, "own-x")
	if err := os.WriteFile(filepath.Join(store.root, corrupt, recordFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	ids, err := store.FindScopeDriveIDs("0342", h)
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if len(got) != 1 || !got[waiting] {
		t.Fatalf("want exactly {waiting}, got %v (finished-owned %s must be excluded)", ids, finishedOwned)
	}
}

// TestFindScopeDriveIDsFinishedDrivesAreNeverCandidates pins change 0489's R1: a
// run whose only drives are a FAILED and a PASSED build drive — both still owned,
// both carrying the run's change id and run context — has NO outer-takeover
// candidate, so run.verdict falls through to its retry path instead of stopping
// takeover-ambiguous. A live drive beside them is the single candidate.
func TestFindScopeDriveIDsFinishedDrivesAreNeverCandidates(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	h := capHash("run-ctx-0489")
	seed := func(outcome Outcome) string {
		t.Helper()
		rec := seedRecord(t)
		rec.ChangeID = "0342"
		rec.RunContextHash = h
		rec.LastOutcome = outcome
		rec.OwnerGeneration = "owner-" + string(outcome) // still owned: never consumed
		id, _, err := store.NewDrive(rec)
		if err != nil {
			t.Fatalf("NewDrive: %v", err)
		}
		return id
	}

	seed(FAILED) // the red gate
	seed(PASSED) // the green re-gate
	ids, err := store.FindScopeDriveIDs("0342", h)
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("finished build drives must never be takeover candidates, got %v", ids)
	}

	live := seed(WAITING)
	ids, err = store.FindScopeDriveIDs("0342", h)
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != live {
		t.Fatalf("a live drive beside finished ones must be the single candidate, got %v want [%s]", ids, live)
	}
}

// TestContinuationHandle proves it returns the current unclaimed handoff token and
// fails typed when the drive carries no unclaimed handoff.
func TestContinuationHandle(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())

	// A plain owned drive has no unclaimed handoff.
	owned, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := store.ContinuationHandle(owned.DriveID); !isOwnershipKind(err, ErrNoHandoffOffered) {
		t.Fatalf("an owned drive must fail ErrNoHandoffOffered, got %v", err)
	}

	// After a handoff the token is the drive's outstanding one.
	handoff, err := d.Handoff(owned.DriveID, owned.Generation)
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	tok, err := store.ContinuationHandle(owned.DriveID)
	if err != nil {
		t.Fatalf("ContinuationHandle: %v", err)
	}
	if tok != handoff.Generation {
		t.Fatalf("ContinuationHandle must return the outstanding handoff token, got %q want %q", tok, handoff.Generation)
	}
}
