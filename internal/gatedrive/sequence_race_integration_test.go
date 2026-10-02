//go:build integration

package gatedrive

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestRaceIntegrationGatedriveSameWorktreeGenerations (spec AC2) drives real generations of
// executions through ONE worktree path over real git and the real process
// supervisor, proving old records for that path never block the next permitted
// drive once release or replacement is proven:
//
//   - drive → release → re-admit across several successive runs: a live
//     run still owns the worktree between drives (a different run is fenced),
//     and once it settles the next run admits over its released slot;
//   - a symlink alias of the worktree reaches the SAME slot and admits;
//   - the live-incumbent counter-case: a genuinely executing run on the canonical
//     path refuses a start through the alias, and admits it once the run finishes;
//   - the worktree is removed while its released slot still names the completed
//     run: retirement reaches the slot through its stored identity (never
//     re-canonicalization of the missing path), and a recreated worktree at the
//     same path admits a new run;
//   - a second remove/recreate leaves the settled run on the inherited slot, and
//     the next (no-run-record) start settles it at admission instead of refusing.
//
// Race shard (change 0466): a live incumbent run holds the worktree slot while a start through its alias contends for it.
func TestRaceIntegrationGatedriveSameWorktreeGenerations(t *testing.T) {
	skipUnlessSupported(t)
	repo := seedSeqRepo(t)
	wt := addLinkedWorktree(t, repo, "wt", "feat/gen")
	store := OpenStore(commonDirOf(t, wt))
	svc := mustService(t)
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
	reapSupervisors(t, runRoot)
	d := realSeqDriver(store, svc)
	runIDs := &genSettled{settled: map[string]bool{}}
	d.SetRunSettledResolver(runIDs.resolve)

	slotOf := func(path string) admissionRecord {
		t.Helper()
		slot, _, err := store.LoadWorktreeExecution(path)
		if err != nil {
			t.Fatalf("load slot for %s: %v", path, err)
		}
		return slot
	}
	passAt := func(path, branch, runID, marker string) DriveDoc {
		t.Helper()
		req := realSeqStart(path, branch, runRoot, "0446", marker, seqPassCmd(marker))
		req.RunID = runID
		doc := driveSeqToTerminal(t, d, req)
		if doc.Outcome != PASSED {
			t.Fatalf("%s must PASS, got %s (%s)", marker, doc.Outcome, doc.Cause)
		}
		return doc
	}

	// 1. Several successive runs on one path.
	lastGen := 0
	for i, runID := range []string{"run-g1", "run-g2", "run-g3"} {
		passAt(wt, "feat/gen", runID, fmt.Sprintf("gen-%d", i+1))
		slot := slotOf(wt)
		if slot.State != admissionReleased || slot.RunID != runID {
			t.Fatalf("after %s: slot %s/%q, want released/%s", runID, slot.State, slot.RunID, runID)
		}
		if slot.ExecutionGen <= lastGen {
			t.Fatalf("after %s: execution generation %d did not advance past %d", runID, slot.ExecutionGen, lastGen)
		}
		lastGen = slot.ExecutionGen
		if i == 0 {
			// Unsettled: the live run owns its worktree between drives.
			foreign := realSeqStart(wt, "feat/gen", runRoot, "0446", "foreign", seqPassCmd("foreign"))
			foreign.RunID = "run-foreign"
			if _, err := d.Start(foreign); !isOwnershipKind(err, ErrStaleRunID) {
				t.Fatalf("an unsettled run must fence a different run, got %v", err)
			}
		}
		runIDs.settle(runID)
	}

	// 2. A symlink alias reaches the same slot and admits (no-run-record start over the
	// settled run-g3's released slot).
	alias := filepath.Join(testsupport.TempDir(t), "wt-alias")
	if err := os.Symlink(wt, alias); err != nil {
		t.Fatalf("symlink alias: %v", err)
	}
	passAt(alias, "feat/gen", "", "gen-alias")
	if slot := slotOf(wt); slot.ExecutionGen != lastGen+1 || slot.RunID != "" {
		t.Fatalf("the alias must reuse the canonical slot: gen %d (want %d) run %q", slot.ExecutionGen, lastGen+1, slot.RunID)
	}

	// 3. Live-incumbent counter-case at the same canonical path.
	release := filepath.Join(testsupport.TempDir(t), "release-live-run")
	liveReq := realSeqStart(wt, "feat/gen", runRoot, "0446", "gen-live",
		[]string{"/bin/sh", "-c", `while [ ! -f "$1" ]; do sleep 0.02; done`, "gen-live", release})
	liveDoc, err := d.Start(liveReq)
	if err != nil || liveDoc.Outcome != WAITING {
		t.Fatalf("live run must start and WAIT: doc=%+v err=%v", liveDoc, err)
	}
	blocked := realSeqStart(alias, "feat/gen", runRoot, "0446", "gen-blocked", seqPassCmd("gen-blocked"))
	if _, err := d.Start(blocked); !isOwnershipKind(err, ErrWorktreeBusy) && !isOwnershipKind(err, ErrLaunchUnconfirmed) {
		t.Fatalf("a genuinely live incumbent must still refuse a start through the alias, got %v", err)
	}
	if slot := slotOf(wt); slot.State != admissionExecuting {
		t.Fatalf("the refused start must leave the live incumbent executing, got %s", slot.State)
	}
	writeFile(t, filepath.Dir(release), filepath.Base(release), "go\n")
	if term, _ := advanceUntilTerminal(t, d, liveDoc.DriveID, liveDoc.Generation); term.Outcome != PASSED {
		t.Fatalf("live run must PASS once released, got %s (%s)", term.Outcome, term.Cause)
	}
	passAt(alias, "feat/gen", "", "gen-after-live")

	// 4. Remove the worktree while its released slot names a completed run; retire
	// through the stored identity; recreate the path; a new run admits.
	passAt(wt, "feat/gen", "run-g4", "gen-4")
	g4 := slotOf(wt)
	git(t, repo, "worktree", "remove", "--force", wt)
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("worktree must be removed, stat err = %v", err)
	}
	removedSlot := slotOf(wt) // stored-identity addressing, not re-canonicalization
	if removedSlot.RunID != "run-g4" || removedSlot.ReservationToken != g4.ReservationToken {
		t.Fatalf("the removed worktree's slot must resolve to its stored record, got run %q", removedSlot.RunID)
	}
	if err := store.RetireWorktreeExecutionRun(wt, "run-g4", g4.ReservationToken); err != nil {
		t.Fatalf("retire a removed worktree's run through its stored identity: %v", err)
	}
	git(t, repo, "worktree", "add", wt, "-b", "feat/gen-r1")
	passAt(wt, "feat/gen-r1", "run-g5", "gen-5")

	// 5. Remove and recreate again WITHOUT retiring: the settled run-g5 left on the
	// inherited released slot is settled at admission, never refused.
	runIDs.settle("run-g5")
	git(t, repo, "worktree", "remove", "--force", wt)
	git(t, repo, "worktree", "add", wt, "-b", "feat/gen-r2")
	passAt(wt, "feat/gen-r2", "", "gen-6")
	if slot := slotOf(wt); slot.RunID != "" || slot.State != admissionReleased {
		t.Fatalf("final slot = %s/%q, want released and run-free", slot.State, slot.RunID)
	}
}
