// Event-authorized parent takeover and continuation-lookup tests. These exercise
// the takeover.go transfer (parent capability, single-use scope claim, child
// owner supersession), Start's scope binding, the cooperative-claim scope close,
// and the two facade-only read surfaces (FindScopeDriveIDs, ContinuationHandle).
// They reuse the deterministic fake clock/proc/git seams from driver_test.go, so
// no test launches a real process or sleeps for a production duration.
package gatedrive

import (
	"encoding/json"
	"fmt"
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
	if scope.CurrentDriveID != started.DriveID {
		t.Fatalf("Start must bind the drive into the scope: CurrentDriveID=%q want %q", scope.CurrentDriveID, started.DriveID)
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

// ---------------------------------------------------------------------------
// Recovery interplay for the sequential scope (change 0405 Task 6): takeover,
// handoff/claim, and enumeration resolve only the scope's CURRENT work — never an
// acknowledged predecessor, a mid-transition slot, or a stale explicit drive id —
// and remain coherent under a race against a successor start or the final
// acknowledgement. These reuse the deterministic fake seams from driver_test.go.
// ---------------------------------------------------------------------------

// overwriteScopeRecord replaces a scope's on-disk record with rec, preserving a
// loadable v2 envelope. Tests use it to inject a precise slot state (e.g. a
// launched slot that still carries a pending-ack journal) without driving the
// transition that would normally produce it.
func overwriteScopeRecord(t *testing.T, store *Store, scopeID string, rec scopeRecord) {
	t.Helper()
	buf, err := json.Marshal(storedScope{Generation: "overwrite-scope-gen", Record: rec})
	if err != nil {
		t.Fatalf("marshal scope: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.scopeRoot, scopeID, recordFileName), buf, 0o600); err != nil {
		t.Fatalf("overwrite scope record: %v", err)
	}
}

// scopedSequenceProc returns a fakeProc where the FIRST launched run (run1) passes
// on its first observation and every later run reports `later` — so a scope's first
// drive PASSES and a successor settles to `later` (StateRunning → WAITING,
// StateFailed → FAILED, StatePassed → PASSED).
func scopedSequenceProc(later process.State) *fakeProc {
	return &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if strings.HasSuffix(runDir, "run1") {
				return obs(process.StatePassed, runDir), nil
			}
			return obs(later, runDir), nil
		},
	}
}

// scopedPredecessorThenSuccessor prepares a no-context task scope over a new store,
// drives a first PASSED drive, then starts a successor over the same slot whose
// outcome is governed by proc's later-run state. It returns the driver, store,
// grant, the base request, and the two drive docs. The current slot occupant is the
// second drive.
func scopedPredecessorThenSuccessor(t *testing.T, proc *fakeProc) (*Driver, *Store, ScopeGrant, StartRequest, DriveDoc, DriveDoc) {
	t.Helper()
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	d := scopedTestDriver(store, clk, proc, stableGit())
	grant, req := prepareScopedStart(t, store)
	first, err := d.Start(req)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if first.Outcome != PASSED {
		t.Fatalf("first start must PASS (positive control), got %s (%s)", first.Outcome, first.Cause)
	}
	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation
	second, err := d.Start(succ)
	if err != nil {
		t.Fatalf("successor Start: %v", err)
	}
	return d, store, grant, req, first, second
}

// TestTakeoverResolvesCurrentNotAcknowledgedPredecessor proves an event-authorized
// takeover after a completed predecessor resolves the CURRENT drive (the live
// successor), never the acknowledged predecessor: the superseded child generation
// can no longer advance, the scope closes, and the predecessor survives as consumed
// history untouched (spec verification 7).
func TestTakeoverResolvesCurrentNotAcknowledgedPredecessor(t *testing.T) {
	proc := scopedSequenceProc(process.StateRunning) // successor stays live (WAITING)
	d, store, grant, _, first, second := scopedPredecessorThenSuccessor(t, proc)
	if second.Outcome != WAITING {
		t.Fatalf("successor must WAIT (live), got %s (%s)", second.Outcome, second.Cause)
	}

	launchesBefore, stopsBefore := proc.launchN, proc.stopN
	took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, "")
	if err != nil {
		t.Fatalf("Takeover: %v", err)
	}
	if took.Outcome == HALTED {
		t.Fatalf("takeover of the current live successor must not HALT: %s", took.Cause)
	}
	if took.DriveID != second.DriveID {
		t.Fatalf("takeover must resolve the CURRENT successor %q, got %q", second.DriveID, took.DriveID)
	}
	if took.DriveID == first.DriveID {
		t.Fatalf("takeover must NOT resolve the acknowledged predecessor")
	}
	if took.Generation == "" || took.Generation == second.Generation {
		t.Fatalf("takeover must mint a fresh owner generation, got %q", took.Generation)
	}
	if proc.launchN != launchesBefore || proc.stopN != stopsBefore {
		t.Fatalf("takeover must not launch or stop a process")
	}

	// The superseded child generation can no longer advance.
	stale, err := d.Advance(second.DriveID, second.Generation)
	if err != nil {
		t.Fatalf("stale Advance: %v", err)
	}
	if stale.Outcome != HALTED {
		t.Fatalf("the superseded child owner must HALT, got %s", stale.Outcome)
	}

	// The scope is closed.
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if !scope.Closed {
		t.Fatalf("a takeover must close the scope")
	}

	// The acknowledged predecessor is untouched consumed history.
	firstRec, err := store.Load(first.DriveID)
	if err != nil {
		t.Fatalf("Load predecessor: %v", err)
	}
	if firstRec.LastOutcome != PASSED || firstRec.OwnerGeneration != "" {
		t.Fatalf("acknowledged predecessor must remain consumed history, got %s owner=%q", firstRec.LastOutcome, firstRec.OwnerGeneration)
	}
}

// TestTakeoverUnacknowledgedTerminalResult proves a takeover resolves the scope's
// current drive when it is an unacknowledged last terminal result (durably FAILED,
// owner still set — the child died after writing the verdict): the takeover reports
// that verdict and closes the scope (spec verification 7).
func TestTakeoverUnacknowledgedTerminalResult(t *testing.T) {
	proc := scopedSequenceProc(process.StateFailed) // successor settles FAILED, unacknowledged
	d, store, grant, _, _, second := scopedPredecessorThenSuccessor(t, proc)
	if second.Outcome != FAILED {
		t.Fatalf("successor must be durably FAILED, got %s (%s)", second.Outcome, second.Cause)
	}

	took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, "")
	if err != nil {
		t.Fatalf("Takeover: %v", err)
	}
	if took.Outcome != FAILED {
		t.Fatalf("takeover of an unacknowledged terminal result must report it, got %s (%s)", took.Outcome, took.Cause)
	}
	if took.DriveID != second.DriveID {
		t.Fatalf("takeover must resolve the current drive %q, got %q", second.DriveID, took.DriveID)
	}
	if took.Generation == "" || took.Generation == second.Generation {
		t.Fatalf("takeover must mint a fresh owner, got %q", took.Generation)
	}
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if !scope.Closed {
		t.Fatalf("a takeover must close the scope")
	}
}

// TestTakeoverExplicitStaleDriveIDRefused proves an explicitly supplied old drive
// id cannot bypass the scope's current-drive association: presenting the
// acknowledged predecessor id to a takeover whose scope now holds a different
// current drive HALTs stale-predecessor, claims nothing, and leaves the current
// drive advanceable (spec verification 4/7).
func TestTakeoverExplicitStaleDriveIDRefused(t *testing.T) {
	proc := scopedSequenceProc(process.StateRunning) // successor live (current)
	d, store, grant, _, first, second := scopedPredecessorThenSuccessor(t, proc)

	launchesBefore, stopsBefore := proc.launchN, proc.stopN
	scopeBefore := readScopeBytes(t, store, grant.ScopeID)

	took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, first.DriveID)
	if err != nil {
		t.Fatalf("Takeover: %v", err)
	}
	if took.Outcome != HALTED {
		t.Fatalf("an explicit stale drive id must HALT, got %s (%s)", took.Outcome, took.Cause)
	}
	if !strings.Contains(took.Cause, string(ErrStalePredecessor)) {
		t.Fatalf("cause must name %q, got %q", ErrStalePredecessor, took.Cause)
	}
	if string(scopeBefore) != string(readScopeBytes(t, store, grant.ScopeID)) {
		t.Fatalf("a rejected takeover must not mutate the scope record")
	}
	if proc.launchN != launchesBefore || proc.stopN != stopsBefore {
		t.Fatalf("a rejected takeover must not launch or stop a process")
	}
	// The current successor's owner is intact: nothing was superseded.
	if _, err := d.Advance(second.DriveID, second.Generation); err != nil {
		t.Fatalf("the current owner must still advance after a rejected takeover: %v", err)
	}
}

// TestTakeoverReservedOrPendingSlotHalts proves a takeover refuses a slot that is
// mid-transition — a reservation not yet launch-confirmed, or a pending-ack journal
// — with a typed unresolved-launch-transition HALT: a reservation is not a
// quiescent result to recover (spec "Durable state and concurrency").
func TestTakeoverReservedOrPendingSlotHalts(t *testing.T) {
	launchFail := func() *fakeProc {
		return &fakeProc{
			launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
				return nil, fmt.Errorf("gatedrive-test: launch failed")
			},
		}
	}

	t.Run("reserved slot via launch failure", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		store := OpenStore(testsupport.TempDir(t))
		grant, req := prepareScopedStart(t, store)
		d := scopedTestDriver(store, clk, launchFail(), stableGit())
		if _, err := d.Start(req); err == nil {
			t.Fatalf("a launch failure must be a command error")
		}
		scope, err := store.LoadScope(grant.ScopeID)
		if err != nil {
			t.Fatalf("LoadScope: %v", err)
		}
		if scope.CurrentDriveState != scopeStateReserved {
			t.Fatalf("precondition: slot must be reserved, got %q", scope.CurrentDriveState)
		}
		took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, "")
		if err != nil {
			t.Fatalf("Takeover: %v", err)
		}
		if took.Outcome != HALTED || !strings.Contains(took.Cause, string(ErrUnresolvedLaunchTransition)) {
			t.Fatalf("a reserved-slot takeover must HALT unresolved-launch-transition, got %s (%s)", took.Outcome, took.Cause)
		}
	})

	t.Run("pending-ack journal on a launched slot", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		store := OpenStore(testsupport.TempDir(t))
		grant, req := prepareScopedStart(t, store)
		d := scopedTestDriver(store, clk, launchFail(), stableGit())
		if _, err := d.Start(req); err == nil {
			t.Fatalf("a launch failure must be a command error")
		}
		// Model an interrupted successor transition: a launched slot still carrying a
		// pending-ack journal entry, so the reserved disjunct is false and this isolates
		// the pending-ack disjunct of the guard.
		scope, err := store.LoadScope(grant.ScopeID)
		if err != nil {
			t.Fatalf("LoadScope: %v", err)
		}
		scope.CurrentDriveState = scopeStateLaunched
		scope.PendingAckDriveID = scope.CurrentDriveID
		scope.PendingAckOwnerGen = "some-owner-gen"
		overwriteScopeRecord(t, store, grant.ScopeID, scope)

		took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, "")
		if err != nil {
			t.Fatalf("Takeover: %v", err)
		}
		if took.Outcome != HALTED || !strings.Contains(took.Cause, string(ErrUnresolvedLaunchTransition)) {
			t.Fatalf("a pending-ack-slot takeover must HALT unresolved-launch-transition, got %s (%s)", took.Outcome, took.Cause)
		}
	})
}

// TestClaimScopeForTakeoverRevalidates unit-tests the claimScopeForTakeover store
// transition directly (revalidate-after-authority): it closes an open scope ONLY
// when the resolved driveID is still the scope's current drive. A concurrent
// successor reservation that advanced the slot to another drive makes the resolved
// id stale, so the close is refused ErrScopeBusy with no write; an outer scope (no
// current drive) closes with any resolved nested id; an already-closed scope is
// ErrScopeClosed; and a takeover close never sets FinalAcked.
func TestClaimScopeForTakeoverRevalidates(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	grant, err := store.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	launchOne(t, store, grant, scopeDriveA) // current = A, launched

	// A drive id that is no longer the slot's current occupant is refused with no write.
	before := readScopeBytes(t, store, grant.ScopeID)
	if err := store.claimScopeForTakeover(grant.ScopeID, scopeDriveB); !isOwnershipKind(err, ErrScopeBusy) {
		t.Fatalf("claimScopeForTakeover on a non-current drive must fail ErrScopeBusy, got %v", err)
	}
	if string(before) != string(readScopeBytes(t, store, grant.ScopeID)) {
		t.Fatalf("a refused claimScopeForTakeover must not rewrite the scope record")
	}

	// The current drive id closes the scope as a takeover (Closed, NOT FinalAcked).
	if err := store.claimScopeForTakeover(grant.ScopeID, scopeDriveA); err != nil {
		t.Fatalf("claimScopeForTakeover on the current drive: %v", err)
	}
	rec, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if !rec.Closed {
		t.Fatalf("claimScopeForTakeover must close the scope")
	}
	if rec.FinalAcked {
		t.Fatalf("a takeover close must not set FinalAcked (that is the terminal ack's mark)")
	}

	// An outer scope (no current drive) closes with any resolved nested id.
	outer := prepareOuterScope(t, store)
	if err := store.claimScopeForTakeover(outer.ScopeID, "ffffffffffffffffffffffffffffffff"); err != nil {
		t.Fatalf("an outer scope must close with any resolved nested id: %v", err)
	}

	// An already-closed scope refuses a second close.
	if err := store.claimScopeForTakeover(grant.ScopeID, scopeDriveA); !isOwnershipKind(err, ErrScopeClosed) {
		t.Fatalf("claimScopeForTakeover on a closed scope must fail ErrScopeClosed, got %v", err)
	}
}

// TestTakeoverRaceVsSuccessorStart reproduces spec verification 5 (successor-start
// vs takeover): a successor start and a parent takeover of the CURRENT drive
// rendezvous at the fingerprint barrier, then contend on the scope. Exactly one
// wins a coherent ownership transition — either the start wins (the takeover HALTs
// scope-busy/scope-closed and supersedes no generation) or the takeover wins (the
// start is rejected ErrScopeClosed) — and at most the winner's Launch happened. Run
// under -race.
func TestTakeoverRaceVsSuccessorStart(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	req := sampleStart()
	grant, err := store.PrepareScope(scopeReqFor(req, ""))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability

	// Setup: a first drive durably PASSED (current, launched, owner set).
	setupClk := &fakeClock{now: startEpoch()}
	setupProc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StatePassed, runDir), nil
		},
	}
	setupDriver := scopedTestDriver(store, setupClk, setupProc, stableGit())
	first, err := setupDriver.Start(req)
	if err != nil {
		t.Fatalf("setup Start: %v", err)
	}
	if first.Outcome != PASSED {
		t.Fatalf("setup first drive must PASS, got %s (%s)", first.Outcome, first.Cause)
	}

	// Race: a successor start (receipt = first) vs a parent takeover of the current
	// (first) drive. barrierGit rendezvouses both past their fingerprint reads before
	// either contends on the scope CAS.
	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation

	proc := &countingProc{}
	var barrier sync.WaitGroup
	barrier.Add(2)
	git := &barrierGit{wg: &barrier, head: "HEAD1"}
	mkDriver := func() *Driver {
		clk := &fakeClock{now: startEpoch()}
		d := NewDriver(store, clk, proc, git)
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		return d
	}
	startD, takeD := mkDriver(), mkDriver()

	var startDoc, takeDoc DriveDoc
	var startErr, takeErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); startDoc, startErr = startD.Start(succ) }()
	go func() { defer wg.Done(); takeDoc, takeErr = takeD.Takeover(grant.ScopeID, grant.ParentCapability, "") }()
	wg.Wait()

	if takeErr != nil {
		t.Fatalf("Takeover returned a command error: %v", takeErr)
	}

	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	firstRec, err := store.Load(first.DriveID)
	if err != nil {
		t.Fatalf("Load first: %v", err)
	}

	startWon := startErr == nil
	takeoverWon := takeDoc.Outcome != HALTED
	if startWon == takeoverWon {
		t.Fatalf("exactly one of start/takeover must win: startWon=%v takeoverWon=%v (startErr=%v takeover=%s/%s)",
			startWon, takeoverWon, startErr, takeDoc.Outcome, takeDoc.Cause)
	}

	if startWon {
		// The successor start wins: it launched exactly once, the scope is NOT closed by
		// the takeover, the slot names the launched successor, and the predecessor was
		// retired by the ack (owner cleared) but no parent generation was superseded.
		if proc.launches() != 1 {
			t.Fatalf("a winning successor start must launch exactly once, got %d", proc.launches())
		}
		if !strings.Contains(takeDoc.Cause, string(ErrScopeBusy)) && !strings.Contains(takeDoc.Cause, string(ErrScopeClosed)) {
			t.Fatalf("a losing takeover must HALT scope-busy or scope-closed, got %q", takeDoc.Cause)
		}
		if scope.Closed {
			t.Fatalf("when the start wins the takeover must not have closed the scope")
		}
		if scope.CurrentDriveID != startDoc.DriveID || scope.CurrentDriveState != scopeStateLaunched {
			t.Fatalf("the slot must name the launched successor, got id=%q state=%q", scope.CurrentDriveID, scope.CurrentDriveState)
		}
		if firstRec.OwnerGeneration != "" {
			t.Fatalf("the predecessor must be retired (owner cleared) by the winning successor, got %q", firstRec.OwnerGeneration)
		}
	} else {
		// The takeover wins: nothing launched, the start is rejected ErrScopeClosed, the
		// scope is closed, and the predecessor's owner is the takeover's fresh generation.
		if proc.launches() != 0 {
			t.Fatalf("when the takeover wins no successor launch must happen, got %d", proc.launches())
		}
		if !isOwnershipKind(startErr, ErrScopeClosed) {
			t.Fatalf("a losing successor start must be rejected ErrScopeClosed, got %v", startErr)
		}
		if !scope.Closed {
			t.Fatalf("when the takeover wins the scope must be closed")
		}
		if takeDoc.Generation == "" || takeDoc.Generation == first.Generation {
			t.Fatalf("a winning takeover must mint a fresh owner, got %q", takeDoc.Generation)
		}
		if firstRec.OwnerGeneration != takeDoc.Generation {
			t.Fatalf("the drive owner must be the takeover's fresh generation, got %q want %q", firstRec.OwnerGeneration, takeDoc.Generation)
		}
	}
}

// gateGit parks its first HeadOID read: it signals readiness once (closing ready)
// and then blocks until release is closed, so a test can drive another transition
// to completion while a takeover is held past its scope/drive reads but before its
// scope claim. Its fixed reads reproduce the stableGit fingerprint so the parked
// takeover's fingerprint check still passes on release.
type gateGit struct {
	ready    chan struct{}
	release  chan struct{}
	head     string
	mu       sync.Mutex
	signaled bool
}

func (g *gateGit) HeadOID(string) (string, error) {
	g.mu.Lock()
	if !g.signaled {
		g.signaled = true
		close(g.ready)
	}
	g.mu.Unlock()
	<-g.release
	return g.head, nil
}
func (g *gateGit) IndexEntries(string) ([]byte, error)  { return []byte("IDX1"), nil }
func (g *gateGit) Status(string) ([]byte, error)        { return []byte("ST1"), nil }
func (g *gateGit) WorktreePaths(string) ([]byte, error) { return nil, nil }

// TestTakeoverRaceVsFinalAcknowledge reproduces spec verification 5 (final-ack vs
// takeover): only one coherent ownership transition wins in each ordering. When the
// takeover completes first it supersedes the owner and the later ack is refused
// stale-predecessor; when the final ack completes while the takeover is parked past
// its reads, the takeover revalidates under its scope claim and HALTs scope-closed.
func TestTakeoverRaceVsFinalAcknowledge(t *testing.T) {
	t.Run("takeover before ack", func(t *testing.T) {
		proc := scopedSequenceProc(process.StatePassed) // successor durably PASSED (current)
		d, store, grant, _, _, second := scopedPredecessorThenSuccessor(t, proc)
		if second.Outcome != PASSED {
			t.Fatalf("successor must be durably PASSED, got %s", second.Outcome)
		}
		took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, "")
		if err != nil {
			t.Fatalf("Takeover: %v", err)
		}
		if took.Outcome == HALTED {
			t.Fatalf("the takeover must win, got HALTED %s", took.Cause)
		}
		// The final acknowledgement can no longer consume this scope: the takeover
		// closed it (Closed, not FinalAcked), so the ack is refused ErrScopeClosed and
		// writes nothing — the takeover, not the ack, owns this terminal transition.
		scopeBytes := readScopeBytes(t, store, grant.ScopeID)
		if _, aerr := d.Acknowledge(grant.ScopeID, grant.ChildCapability, second.DriveID, second.Generation); !isOwnershipKind(aerr, ErrScopeClosed) {
			t.Fatalf("an ack after a takeover closed the scope must fail ErrScopeClosed, got %v", aerr)
		}
		if string(scopeBytes) != string(readScopeBytes(t, store, grant.ScopeID)) {
			t.Fatalf("a refused ack must not rewrite the scope record")
		}
		scope, err := store.LoadScope(grant.ScopeID)
		if err != nil {
			t.Fatalf("LoadScope: %v", err)
		}
		if !scope.Closed || scope.FinalAcked {
			t.Fatalf("a takeover close must leave the scope Closed and NOT FinalAcked, got Closed=%v FinalAcked=%v", scope.Closed, scope.FinalAcked)
		}
	})

	t.Run("ack completes while a takeover is parked past its reads", func(t *testing.T) {
		store := OpenStore(testsupport.TempDir(t))
		req := sampleStart()
		grant, err := store.PrepareScope(scopeReqFor(req, ""))
		if err != nil {
			t.Fatalf("PrepareScope: %v", err)
		}
		req.ScopeID = grant.ScopeID
		req.ChildCapability = grant.ChildCapability

		setupDriver := scopedTestDriver(store, &fakeClock{now: startEpoch()}, passObserveProc(), stableGit())
		started, err := setupDriver.Start(req)
		if err != nil {
			t.Fatalf("setup Start: %v", err)
		}
		if started.Outcome != PASSED {
			t.Fatalf("setup drive must PASS, got %s", started.Outcome)
		}

		gate := &gateGit{ready: make(chan struct{}), release: make(chan struct{}), head: "HEAD1"}
		takeClk := &fakeClock{now: startEpoch()}
		takeDriver := NewDriver(store, takeClk, &fakeProc{}, gate)
		takeDriver.slice = 4 * pollTick
		takeDriver.pollInterval = pollTick
		takeDriver.sleep = func(dur time.Duration) { takeClk.advance(dur) }

		var takeDoc DriveDoc
		var takeErr error
		done := make(chan struct{})
		go func() {
			defer close(done)
			takeDoc, takeErr = takeDriver.Takeover(grant.ScopeID, grant.ParentCapability, "")
		}()

		<-gate.ready // the takeover is parked past its scope/drive reads, before its close

		ackDriver := scopedTestDriver(store, &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit())
		ackDoc, aerr := ackDriver.Acknowledge(grant.ScopeID, grant.ChildCapability, started.DriveID, started.Generation)
		if aerr != nil {
			t.Fatalf("Acknowledge: %v", aerr)
		}
		if ackDoc.Outcome != PASSED {
			t.Fatalf("ack must report PASSED, got %s", ackDoc.Outcome)
		}

		close(gate.release) // release the parked takeover
		<-done

		if takeErr != nil {
			t.Fatalf("Takeover returned a command error: %v", takeErr)
		}
		if takeDoc.Outcome != HALTED || !strings.Contains(takeDoc.Cause, string(ErrScopeClosed)) {
			t.Fatalf("a takeover racing a completed ack must HALT scope-closed, got %s (%s)", takeDoc.Outcome, takeDoc.Cause)
		}
		scope, err := store.LoadScope(grant.ScopeID)
		if err != nil {
			t.Fatalf("LoadScope: %v", err)
		}
		if !scope.Closed || !scope.FinalAcked {
			t.Fatalf("the ack must have closed the scope FinalAcked, got Closed=%v FinalAcked=%v", scope.Closed, scope.FinalAcked)
		}
	})
}

// TestFindScopeDriveIDsExcludesAcknowledgedHistory pins spec verification 7's tail:
// outer discovery over a change with two acknowledged predecessors (terminal,
// owner-cleared) and one current WAITING drive resolves to exactly the current one,
// never a crowd of consumed historical results from the same sequence.
func TestFindScopeDriveIDsExcludesAcknowledgedHistory(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	seed := func(outcome Outcome, owner string) string {
		t.Helper()
		rec := seedRecord(t)
		rec.ChangeID = "0342"
		rec.GateContextHash = capHash("seq-ctx")
		rec.LastOutcome = outcome
		rec.OwnerGeneration = owner
		id, _, err := store.NewDrive(rec)
		if err != nil {
			t.Fatalf("NewDrive: %v", err)
		}
		return id
	}
	seed(PASSED, "")                        // acknowledged predecessor 1 (consumed)
	seed(PASSED, "")                        // acknowledged predecessor 2 (consumed)
	current := seed(WAITING, "own-current") // the current live drive

	ids, err := store.FindScopeDriveIDs("0342", capHash("seq-ctx"))
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != current {
		t.Fatalf("outer discovery must find exactly the current drive %q, got %v", current, ids)
	}
}
