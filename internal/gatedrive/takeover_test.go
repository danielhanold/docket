// Event-authorized parent takeover and continuation-lookup tests. These exercise
// the takeover.go transfer (parent capability, single-use scope claim, child
// owner supersession), Start's scope binding, the cooperative-claim scope close,
// and the two facade-only read surfaces (FindScopeDriveIDs, ContinuationHandle).
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

// scopeReqFor builds a ScopeRequest whose identity matches a StartRequest, so a
// drive Started under the grant binds cleanly. gateContext is the raw outer
// child-context token (empty for a plain task scope in these tests).
func scopeReqFor(req StartRequest, gateContext string) ScopeRequest {
	return ScopeRequest{
		RepoIdentity: req.RepoDir,
		ChangeID:     req.ChangeID,
		TaskID:       req.TaskID,
		Phase:        req.Phase,
		Branch:       req.Branch,
		Worktree:     req.Worktree,
		GateContext:  gateContext,
	}
}

// bindWaiting prepares a task scope, Starts a scope-bound drive under it, and
// asserts the first slice WAITs. It returns the grant and the WAITING doc.
func bindWaiting(t *testing.T, d *Driver, store *Store) (ScopeGrant, DriveDoc) {
	t.Helper()
	req := sampleStart()
	grant, err := store.PrepareScope(scopeReqFor(req, ""))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	started, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.Outcome != WAITING {
		t.Fatalf("scope-bound first slice must WAIT, got %s (%s)", started.Outcome, started.Cause)
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
// takeover of a WAITING scope-bound drive returns a NEW owner generation, the old
// child owner can no longer advance, the new owner advances normally, the scope
// is closed, and the takeover itself neither launched nor stopped any process.
func TestTakeoverInvalidatesChildAndMintsParentOwner(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{} // stays running across slices
	d, store := newTestDriver(t, clk, proc, stableGit())

	grant, started := bindWaiting(t, d, store)

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

// TestTakeoverTerminalUnconsumed proves a takeover of an already-PASSED drive
// whose owner generation is still set (the child died after writing the verdict)
// succeeds, and the fresh owner's Advance returns the recorded PASSED with the
// same attempt and no relaunch.
func TestTakeoverTerminalUnconsumed(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
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
	grant, started := bindWaiting(t, d, store)

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
				grant, started := bindWaiting(t, d, store)
				return grant.ScopeID, bogusCap, started.DriveID
			},
			want: string(ErrScopeCapabilityMismatch),
		},
		{
			name: "child capability presented as parent",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := bindWaiting(t, d, store)
				return grant.ScopeID, grant.ChildCapability, started.DriveID
			},
			want: string(ErrScopeCapabilityMismatch),
		},
		{
			name: "closed scope",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := bindWaiting(t, d, store)
				if err := store.closeScope(grant.ScopeID); err != nil {
					t.Fatalf("closeScope: %v", err)
				}
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: string(ErrScopeClosed),
		},
		{
			name: "identity mismatch branch",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := bindWaiting(t, d, store)
				rec, _ := store.Load(started.DriveID)
				rec.Branch = "feat/other"
				overwriteDriveRecord(t, store, started.DriveID, rec)
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: "identity-mismatch",
		},
		{
			name: "identity mismatch worktree",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := bindWaiting(t, d, store)
				rec, _ := store.Load(started.DriveID)
				rec.WorktreePath = "/repo/other"
				overwriteDriveRecord(t, store, started.DriveID, rec)
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: "identity-mismatch",
		},
		{
			name: "identity mismatch change",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := bindWaiting(t, d, store)
				rec, _ := store.Load(started.DriveID)
				rec.ChangeID = "9999"
				overwriteDriveRecord(t, store, started.DriveID, rec)
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: "identity-mismatch",
		},
		{
			name: "identity mismatch task",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := bindWaiting(t, d, store)
				rec, _ := store.Load(started.DriveID)
				rec.TaskID = "task-other"
				overwriteDriveRecord(t, store, started.DriveID, rec)
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: "identity-mismatch",
		},
		{
			name: "identity mismatch phase",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := bindWaiting(t, d, store)
				rec, _ := store.Load(started.DriveID)
				rec.Phase = "finalize"
				overwriteDriveRecord(t, store, started.DriveID, rec)
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: "identity-mismatch",
		},
		{
			name: "fingerprint drift",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := bindWaiting(t, d, store)
				git.status = "DRIFTED" // the worktree changed since drive start
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: string(ErrFingerprintMismatch),
		},
		{
			name: "expired deadline on a live drive",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := bindWaiting(t, d, store)
				rec, _ := store.Load(started.DriveID)
				rec.Deadline = startEpoch().Add(-time.Minute) // past, drive still WAITING
				overwriteDriveRecord(t, store, started.DriveID, rec)
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: CauseDeadlineExpired,
		},
		{
			name: "outstanding unclaimed handoff must be claimed",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := bindWaiting(t, d, store)
				if _, err := d.Handoff(started.DriveID, started.Generation); err != nil {
					t.Fatalf("Handoff: %v", err)
				}
				return grant.ScopeID, grant.ParentCapability, started.DriveID
			},
			want: string(ErrHandoffOutstanding),
		},
		{
			name: "two candidate drives for one outer scope",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant := prepareOuterScope(t, store)
				startNested(t, d, grant.ChildCapability)
				startNested(t, d, grant.ChildCapability)
				return grant.ScopeID, grant.ParentCapability, "" // resolve via gate context
			},
			want: CauseTakeoverAmbiguous,
		},
		{
			name: "zero candidate drives",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant := prepareOuterScope(t, store)
				return grant.ScopeID, grant.ParentCapability, ""
			},
			want: CauseTakeoverNoCandidate,
		},
		{
			name: "unknown drive schema",
			setup: func(t *testing.T, d *Driver, store *Store, git *fakeGit) (string, string, string) {
				grant, started := bindWaiting(t, d, store)
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
			clk := &fakeClock{now: startEpoch()}
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

// prepareOuterScope prepares an outer recovery scope (no bound drive) whose change
// and child capability are the gate-context discriminators nested drives carry.
func prepareOuterScope(t *testing.T, store *Store) ScopeGrant {
	t.Helper()
	grant, err := store.PrepareScope(ScopeRequest{
		RepoIdentity: "/repo",
		ChangeID:     "0342",
		Branch:       "feat/x",
		Worktree:     "/repo",
	})
	if err != nil {
		t.Fatalf("PrepareScope outer: %v", err)
	}
	return grant
}

// startNested Starts a nested (non-scope-bound) drive whose GateContext is the
// outer scope's child capability, so its GateContextHash matches the outer scope.
func startNested(t *testing.T, d *Driver, gateContext string) DriveDoc {
	t.Helper()
	req := sampleStart()
	req.GateContext = gateContext
	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start nested: %v", err)
	}
	return doc
}

// TestTakeoverRace proves two goroutines racing Takeover with the same parent
// capability yield exactly one fresh owner; the loser HALTs and the old child
// owner is invalid either way. Run under -race.
func TestTakeoverRace(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	grant, started := bindWaiting(t, d, store)

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

// TestStartBindsScope proves Start with ScopeID+ChildCapability binds the drive
// into the scope and stamps ScopeID + GateContextHash; a wrong capability fails
// BEFORE launch; a second Start on the same scope while the first drive is live
// fails.
func TestStartBindsScope(t *testing.T) {
	const gateCtx = "outer-dispatch-context-token"

	// Happy path: binds + stamps.
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	req := sampleStart()
	grant, err := store.PrepareScope(scopeReqFor(req, gateCtx))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	req.GateContext = gateCtx
	started, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.BoundDriveID != started.DriveID {
		t.Fatalf("Start must bind the drive into the scope: BoundDriveID=%q want %q", scope.BoundDriveID, started.DriveID)
	}
	rec, err := store.Load(started.DriveID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.ScopeID != grant.ScopeID {
		t.Fatalf("Start must stamp ScopeID, got %q", rec.ScopeID)
	}
	if rec.GateContextHash != capHash(gateCtx) {
		t.Fatalf("Start must stamp the gate-context hash, got %q", rec.GateContextHash)
	}

	// A second Start on the same (already-bound) scope fails while the first is live.
	dup := sampleStart()
	dup.ScopeID = grant.ScopeID
	dup.ChildCapability = grant.ChildCapability
	launchesBefore := proc.launchN
	if _, err := d.Start(dup); !isOwnershipKind(err, ErrScopeSecondDrive) {
		t.Fatalf("a second Start on a bound scope must fail ErrScopeSecondDrive, got %v", err)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a rejected second Start must not launch, launched %d->%d", launchesBefore, proc.launchN)
	}

	// A wrong capability fails BEFORE launch on a fresh scope.
	clk2 := &fakeClock{now: startEpoch()}
	proc2 := &fakeProc{}
	d2, store2 := newTestDriver(t, clk2, proc2, stableGit())
	req2 := sampleStart()
	grant2, err := store2.PrepareScope(scopeReqFor(req2, ""))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req2.ScopeID = grant2.ScopeID
	req2.ChildCapability = "wrong-capability"
	if _, err := d2.Start(req2); !isOwnershipKind(err, ErrScopeCapabilityMismatch) {
		t.Fatalf("a wrong capability must fail ErrScopeCapabilityMismatch, got %v", err)
	}
	if proc2.launchN != 0 {
		t.Fatalf("a wrong capability must fail BEFORE launch, launched %d", proc2.launchN)
	}
}

// TestFindScopeDriveIDs proves the outer-gate candidate resolver: it lists drives
// matching change + gate-context hash that are nonterminal OR terminal-unconsumed,
// excludes a terminal-consumed drive, and skips unreadable records.
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
		rec.GateContextHash = gateHash
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
	termUnconsumed := seed("0342", h, PASSED, "own-t") // terminal, owner still set
	seed("0342", h, PASSED, "")                        // terminal AND consumed → excluded
	seed("0342", other, WAITING, "own-o")              // wrong gate context → excluded
	seed("0400", h, WAITING, "own-c")                  // wrong change → excluded

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
	if len(got) != 2 || !got[waiting] || !got[termUnconsumed] {
		t.Fatalf("want exactly {waiting, terminal-unconsumed}, got %v", ids)
	}
}

// TestContinuationHandle proves it returns the current unclaimed handoff token and
// fails typed when the drive carries no unclaimed handoff.
func TestContinuationHandle(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
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
