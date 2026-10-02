//go:build integration

package gatedrive

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestRaceIntegrationGatedriveSameWorktreeGenerations (spec AC2, re-targeted onto
// the worktree lock by change 0490) drives real generations of executions
// through ONE worktree path over real git and the real process supervisor,
// proving a finished gate never blocks the next drive and nothing has to be
// recovered between them:
//
//   - several successive drives on one path — under different runs, as a resumed
//     or replacement build would bring, and under none — each admit and PASS;
//   - a symlink alias of the worktree reaches the SAME lock and admits;
//   - the live-incumbent counter-case: a genuinely executing run on the canonical
//     path refuses a start through the alias worktree-busy, naming the live
//     holder, and admits it once the run finishes;
//   - the worktree is removed and recreated at the same path: a new drive admits.
//
// The slot-era legs (a live run fencing other runs between its drives, slot
// generations, and retiring a removed worktree's slot) are retired with the slot.
//
// Race shard (change 0466): a live incumbent run holds the worktree while a start through its alias contends for it.
func TestRaceIntegrationGatedriveSameWorktreeGenerations(t *testing.T) {
	skipUnlessSupported(t)
	repo := seedSeqRepo(t)
	wt := addLinkedWorktree(t, repo, "wt", "feat/gen")
	store := OpenStore(commonDirOf(t, wt))
	svc := mustService(t)
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")
	reapSupervisors(t, runRoot)
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
	d := realSeqDriver(store, svc)

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

	// 1. Several successive drives on one path, under different runs and none.
	for i, runID := range []string{"run-g1", "run-g2", "", "run-g3"} {
		passAt(wt, "feat/gen", runID, fmt.Sprintf("gen-%d", i+1))
	}

	// 2. A symlink alias reaches the same lock and admits.
	alias := filepath.Join(testsupport.TempDir(t), "wt-alias")
	if err := os.Symlink(wt, alias); err != nil {
		t.Fatalf("symlink alias: %v", err)
	}
	passAt(alias, "feat/gen", "", "gen-alias")

	// 3. Live-incumbent counter-case at the same canonical path.
	release := filepath.Join(testsupport.TempDir(t), "release-live-run")
	liveReq := realSeqStart(wt, "feat/gen", runRoot, "0446", "gen-live",
		[]string{"/bin/sh", "-c", `while [ ! -f "$1" ]; do sleep 0.02; done`, "gen-live", release})
	liveDoc, err := d.Start(liveReq)
	if err != nil || liveDoc.Outcome != WAITING {
		t.Fatalf("live run must start and WAIT: doc=%+v err=%v", liveDoc, err)
	}
	blocked := realSeqStart(alias, "feat/gen", runRoot, "0446", "gen-blocked", seqPassCmd("gen-blocked"))
	_, err = d.Start(blocked)
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Kind != ErrWorktreeBusy {
		t.Fatalf("a genuinely live incumbent must refuse a start through the alias, got %v", err)
	}
	if oe.Incumbent == nil || oe.Incumbent.DriveID != liveDoc.DriveID {
		t.Fatalf("the refusal must name the live holder %s, got %+v", liveDoc.DriveID, oe.Incumbent)
	}
	writeFile(t, filepath.Dir(release), filepath.Base(release), "go\n")
	if term, _ := advanceUntilTerminal(t, d, liveDoc.DriveID, liveDoc.Generation); term.Outcome != PASSED {
		t.Fatalf("live run must PASS once released, got %s (%s)", term.Outcome, term.Cause)
	}
	passAt(alias, "feat/gen", "", "gen-after-live")

	// 4. Remove the worktree and recreate it at the same path: a new drive admits.
	git(t, repo, "worktree", "remove", "--force", wt)
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("worktree must be removed, stat err = %v", err)
	}
	git(t, repo, "worktree", "add", wt, "-b", "feat/gen-r1")
	passAt(wt, "feat/gen-r1", "run-g5", "gen-5")
}
