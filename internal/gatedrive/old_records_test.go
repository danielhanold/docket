// Records a pre-0489 binary left on disk (change 0489, Decision 4). There is no
// schema bump, store reset, or migration: an outer scope keeps its on-disk shape,
// a task scope is never opened again, and an old task drive still carrying
// scope_id decodes as a scopeless drive. These tests pin that the leftovers stay
// inert evidence rather than blocking a live run.
package gatedrive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// writeRawScope writes a scope record exactly as a pre-0489 binary left it on
// disk: the storedScope envelope around an arbitrary record map.
func writeRawScope(t *testing.T, store *Store, id string, record map[string]any) {
	t.Helper()
	dir := filepath.Join(store.scopeRoot, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir scope: %v", err)
	}
	buf, err := json.Marshal(map[string]any{"generation": "pre-0489-gen", "record": record})
	if err != nil {
		t.Fatalf("marshal scope: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, recordFileName), buf, 0o600); err != nil {
		t.Fatalf("write scope: %v", err)
	}
}

// stampLegacyScopeID rewrites drive id's persisted record to carry scope_id, the
// field a pre-0489 task drive recorded and the current decoder ignores, so the
// record on disk is byte-for-byte the shape an old binary left.
func stampLegacyScopeID(t *testing.T, store *Store, id, scopeID string) {
	t.Helper()
	path := filepath.Join(store.root, id, recordFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	env["record"].(map[string]any)["scope_id"] = scopeID
	if raw, err = json.Marshal(env); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestOldTaskScopeOnDiskChangesNothing (change 0489, Decision 4; Review Focus 2):
// an OPEN task scope a pre-0489 binary left on disk — slot fields, a pending-ack
// journal, and a run id naming the run under test — is never read, so it blocks
// neither cancellation's census, the successful-run closeout census, nor admission.
func TestOldTaskScopeOnDiskChangesNothing(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	d, store := newTestDriver(t, clk, &fakeProc{}, stableGit())
	const runID = "run-0489-old-task-scope"
	writeRawScope(t, store, "abababababababababababababababab", map[string]any{
		"schema_version": 3, "repo_identity": "/repo", "change_id": "0342",
		"task_id": "task-6", "phase": "build", "branch": "feat/x", "worktree": sampleWorktree(),
		"child_cap_hash": capHash("old-child"), "parent_cap_hash": capHash("old-parent"),
		"run_id":               runID,
		"current_drive_id":     "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd",
		"current_drive_state":  "reserved",
		"pending_ack_drive_id": "efefefefefefefefefefefefefefefef", "pending_ack_owner_gen": "old-gen",
		"drive_count": 2, "closed": false,
	})

	// The census attributes by run context (change 0490); even the old scope's own
	// child context names no drive, because the census never opens a scope.
	for name, census := range map[string]func(string) (RunLaunchReport, error){
		"cancel":   d.ReconcileRunLaunches,
		"closeout": d.ObserveRunLaunches,
	} {
		rep, err := census(capHash("old-child"))
		if err != nil {
			t.Fatalf("%s census: %v", name, err)
		}
		if !rep.Accounted || len(rep.Findings) != 0 {
			t.Fatalf("%s census over an old task scope = accounted %v findings %v, want accounted with no findings", name, rep.Accounted, rep.Findings)
		}
	}

	req := sampleStart()
	req.RunID = runID
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("admission beside an old task scope must succeed: %v", err)
	}
	_ = d.AbandonAdmission(ticket)
}

// TestOldOuterScopeOnDiskStillLoadsBindsAndTakesOver (change 0489, Decision 4;
// Review Focus 1): an outer scope written in the pre-0489 on-disk shape — every
// field the old binary wrote, including the ones this change drops — still loads,
// binds its change, and authorizes a takeover of the run's live drive, so a run in
// flight across the upgrade keeps its continuation path.
func TestOldOuterScopeOnDiskStillLoadsBindsAndTakesOver(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	d, store := newTestDriver(t, clk, &fakeProc{}, stableGit())
	req := sampleStart()
	const runContext = "pre-0489-run-context"
	const parentCap = "pre-0489-parent-capability"
	const scopeID = "abababababababababababababababab"
	writeRawScope(t, store, scopeID, map[string]any{
		"schema_version": 3, "repo_identity": req.RepoDir, "change_id": "",
		"task_id": "", "phase": "", "branch": req.Branch, "worktree": req.Worktree,
		"child_cap_hash": capHash(runContext), "parent_cap_hash": capHash(parentCap),
		"drive_count": 0, "closed": false,
	})
	if _, err := store.LoadScope(scopeID); err != nil {
		t.Fatalf("LoadScope of a pre-0489 outer scope: %v", err)
	}
	if err := d.BindScopeChange(scopeID, req.ChangeID); err != nil {
		t.Fatalf("BindScopeChange: %v", err)
	}
	req.RunContext = runContext
	started, err := d.Start(req)
	if err != nil || started.Outcome != WAITING {
		t.Fatalf("Start under the old outer scope: %v (%s)", err, started.Outcome)
	}
	ids, err := store.FindScopeDriveIDs(req.ChangeID, capHash(runContext))
	if err != nil || len(ids) != 1 || ids[0] != started.DriveID {
		t.Fatalf("outer scan = %v, %v; want [%s]", ids, err, started.DriveID)
	}
	took, err := d.Takeover(scopeID, parentCap, started.DriveID)
	if err != nil || took.Outcome == HALTED {
		t.Fatalf("takeover under a pre-0489 outer scope: err=%v outcome=%s cause=%q", err, took.Outcome, took.Cause)
	}
}

// TestOldDriveRecordWithScopeIDSettlesScopeless (change 0489, Decision 4; Review
// Focus 3): a pre-0489 task drive record still carrying scope_id — naming a scope
// that is not on disk — decodes as a scopeless drive, never run-record-unreadable.
// The census attributes it by its run context like any other drive (change 0490):
// a drive of the run whose run dir is gone is torn down and does not block the run.
func TestOldDriveRecordWithScopeIDSettlesScopeless(t *testing.T) {
	t.Run("old-task-drive-of-the-run-settles-by-context", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		d, store := newTestDriver(t, clk, &fakeProc{}, stableGit())
		const runContext = "run-0489-old-task-drive-context"
		id, _ := seedRunDrive(t, store, runContext, func(r *driveRecord) {
			r.LastOutcome = WAITING
			r.RawRunDir = filepath.Join(testsupport.TempDir(t), "removed-run")
		})
		stampLegacyScopeID(t, store, id, "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd")

		if _, err := store.Load(id); err != nil {
			t.Fatalf("Load of an old drive carrying scope_id: %v", err)
		}
		rep, err := d.ReconcileRunLaunches(capHash(runContext))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !rep.Accounted {
			t.Fatalf("an old task drive whose run is gone must not block the run, findings=%v", rep.Findings)
		}
		if reconcileFindingPresent(rep.Findings, "record-unreadable:") || reconcileFindingPresent(rep.Findings, "history-unattributed:") {
			t.Fatalf("an old drive carrying scope_id is a readable drive of the run, findings=%v", rep.Findings)
		}
	})

	t.Run("old-drive-without-run-context-is-never-attributed", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		rec := seedRecord(t)
		rec.LastOutcome = WAITING
		id, _, err := store.NewDrive(rec)
		if err != nil {
			t.Fatalf("NewDrive: %v", err)
		}
		stampLegacyScopeID(t, store, id, "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd")

		if _, err := store.Load(id); err != nil {
			t.Fatalf("Load of an old drive carrying scope_id: %v", err)
		}
		rep, err := d.ReconcileRunLaunches(capHash("run-0489-any"))
		if err != nil || !rep.Accounted {
			t.Fatalf("census over an old scope_id drive = %+v, %v; want accounted", rep, err)
		}
		if proc.observeN != 0 || proc.stopN != 0 {
			t.Fatalf("a drive with no run context is never attributed: observed %d, stopped %d", proc.observeN, proc.stopN)
		}
	})
}
