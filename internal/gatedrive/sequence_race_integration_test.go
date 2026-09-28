//go:build integration

package gatedrive

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestRaceIntegrationGatedriveSequenceConcurrentScopesResolveOwnWork runs two scopes in two
// linked worktrees that share ONE git common dir, concurrently, with
// distinguishable commands and opposite verdicts, and proves each parent's
// takeover and enumeration resolve only its own scope's current drive — repeated
// across two tasks of one change, two different changes, and with acknowledged
// historical drives present in the store. Concurrency is real (two goroutines
// driving the shared store); -race is the oracle for cross-scope interference.
// Race shard (change 0466): two goroutines drive distinct scopes to terminal concurrently.
func TestRaceIntegrationGatedriveSequenceConcurrentScopesResolveOwnWork(t *testing.T) {
	skipUnlessSupported(t)
	repo := seedSeqRepo(t)
	wtA := addLinkedWorktree(t, repo, "wtA", "feat/a")
	wtB := addLinkedWorktree(t, repo, "wtB", "feat/b")
	common := commonDirOf(t, wtA)
	if other := commonDirOf(t, wtB); other != common {
		t.Fatalf("two linked worktrees must share ONE git common dir: %q vs %q", common, other)
	}
	store := OpenStore(common)
	svc := mustService(t)

	// Each scope launches under its OWN process run root: the native supervisor's
	// registry lock is a per-root non-blocking allocation probe (internal/process
	// lock.go LOCK_EX|LOCK_NB), so two concurrent launches sharing one root would
	// contend — and in production each parent picks its own scratch run root anyway.
	// The SHARED resource under test is the git common dir (one drive/scope store),
	// not the process allocation root.
	scopeRunRoot := func(t *testing.T) string {
		t.Helper()
		rr := filepath.Join(testsupport.TempDir(t), "runs")
		t.Cleanup(func() { stopAllRuns(t, svc, rr) })
		reapSupervisors(t, rr)
		return rr
	}

	// prep prepares a task scope bound to a worktree and its own run root, and returns
	// the grant plus a start request wired to it (scope id + child capability + gate
	// context).
	prep := func(t *testing.T, wt, branch, changeID, taskID, gateCtx string, cmd []string) (ScopeGrant, StartRequest) {
		t.Helper()
		req := realSeqStart(wt, branch, scopeRunRoot(t), changeID, taskID, cmd)
		grant, err := store.PrepareScope(scopeReqFor(req, gateCtx))
		if err != nil {
			t.Fatalf("PrepareScope: %v", err)
		}
		req.ScopeID = grant.ScopeID
		req.ChildCapability = grant.ChildCapability
		req.GateContext = gateCtx
		return grant, req
	}

	// runPair drives reqA and reqB concurrently to their terminals over the shared
	// store, joining before it returns the two docs.
	runPair := func(t *testing.T, dA, dB *Driver, reqA, reqB StartRequest) (DriveDoc, DriveDoc) {
		t.Helper()
		var wg sync.WaitGroup
		var docA, docB DriveDoc
		var errA, errB error
		wg.Add(2)
		go func() { defer wg.Done(); docA, errA = driveSeqToTerminalErr(dA, reqA) }()
		go func() { defer wg.Done(); docB, errB = driveSeqToTerminalErr(dB, reqB) }()
		wg.Wait()
		if errA != nil {
			t.Fatalf("scope A concurrent drive: %v", errA)
		}
		if errB != nil {
			t.Fatalf("scope B concurrent drive: %v", errB)
		}
		return docA, docB
	}

	// assertResolvesOwn proves each scope's enumeration and each parent's takeover
	// resolve ONLY that scope's own current drive. Enumeration is asserted before the
	// takeovers, which close each scope and mint fresh parent owners.
	assertResolvesOwn := func(t *testing.T, dA, dB *Driver,
		gA ScopeGrant, changeA, gateA, driveA string,
		gB ScopeGrant, changeB, gateB, driveB string) {
		t.Helper()
		if driveA == driveB {
			t.Fatalf("the two concurrent scopes must resolve DISTINCT drives, both %q", driveA)
		}
		idsA, err := store.FindScopeDriveIDs(changeA, capHash(gateA))
		if err != nil {
			t.Fatalf("FindScopeDriveIDs A: %v", err)
		}
		if !idsContain(idsA, driveA) || idsContain(idsA, driveB) {
			t.Fatalf("scope A enumeration must resolve only its own drive %q, got %v", driveA, idsA)
		}
		idsB, err := store.FindScopeDriveIDs(changeB, capHash(gateB))
		if err != nil {
			t.Fatalf("FindScopeDriveIDs B: %v", err)
		}
		if !idsContain(idsB, driveB) || idsContain(idsB, driveA) {
			t.Fatalf("scope B enumeration must resolve only its own drive %q, got %v", driveB, idsB)
		}

		tookA, err := dA.Takeover(gA.ScopeID, gA.ParentCapability, "")
		if err != nil {
			t.Fatalf("Takeover A: %v", err)
		}
		if tookA.DriveID != driveA {
			t.Fatalf("scope A takeover must resolve its own current drive %q, got %q", driveA, tookA.DriveID)
		}
		tookB, err := dB.Takeover(gB.ScopeID, gB.ParentCapability, "")
		if err != nil {
			t.Fatalf("Takeover B: %v", err)
		}
		if tookB.DriveID != driveB {
			t.Fatalf("scope B takeover must resolve its own current drive %q, got %q", driveB, tookB.DriveID)
		}
		if tookA.DriveID == tookB.DriveID {
			t.Fatalf("the two parents' takeovers must resolve distinct drives")
		}
	}

	t.Run("two tasks of one change", func(t *testing.T) {
		gA, rA := prep(t, wtA, "feat/a", "0540", "task-1", "gate-0540-a", seqPassCmd("2tasks-A"))
		gB, rB := prep(t, wtB, "feat/b", "0540", "task-2", "gate-0540-b", seqFailCmd("2tasks-B"))
		dA, dB := realSeqDriver(store, svc), realSeqDriver(store, svc)
		docA, docB := runPair(t, dA, dB, rA, rB)
		if docA.Outcome != PASSED {
			t.Fatalf("scope A must PASS, got %s (%s)", docA.Outcome, docA.Cause)
		}
		if docB.Outcome != FAILED {
			t.Fatalf("scope B must FAIL (opposite verdict), got %s (%s)", docB.Outcome, docB.Cause)
		}
		assertCommandMarker(t, mustLoad(t, store, docA.DriveID), "2tasks-A")
		assertCommandMarker(t, mustLoad(t, store, docB.DriveID), "2tasks-B")
		assertResolvesOwn(t, dA, dB, gA, "0540", "gate-0540-a", docA.DriveID, gB, "0540", "gate-0540-b", docB.DriveID)
	})

	t.Run("two different changes", func(t *testing.T) {
		gA, rA := prep(t, wtA, "feat/a", "0541", "task-1", "gate-0541-a", seqPassCmd("2chg-A"))
		gB, rB := prep(t, wtB, "feat/b", "0542", "task-1", "gate-0542-b", seqFailCmd("2chg-B"))
		dA, dB := realSeqDriver(store, svc), realSeqDriver(store, svc)
		docA, docB := runPair(t, dA, dB, rA, rB)
		if docA.Outcome != PASSED {
			t.Fatalf("scope A must PASS, got %s (%s)", docA.Outcome, docA.Cause)
		}
		if docB.Outcome != FAILED {
			t.Fatalf("scope B must FAIL (opposite verdict), got %s (%s)", docB.Outcome, docB.Cause)
		}
		assertResolvesOwn(t, dA, dB, gA, "0541", "gate-0541-a", docA.DriveID, gB, "0542", "gate-0542-b", docB.DriveID)
	})

	t.Run("with acknowledged historical drives present", func(t *testing.T) {
		// Build acked history under change 0543 / gate-0543-a on wtA: a
		// baseline+successor sequence, terminally acknowledged, leaving two
		// owner-cleared historical drives in the shared store.
		ackedIDs := makeAckedHistory(t, realSeqDriver(store, svc), store, wtA, "feat/a", scopeRunRoot(t), "0543", "task-1", "gate-0543-a")

		// A fresh CURRENT scope under the SAME change+gate must still resolve to
		// exactly its own current drive despite the acked history.
		gA, rA := prep(t, wtA, "feat/a", "0543", "task-1", "gate-0543-a", seqPassCmd("acked-current-A"))
		gB, rB := prep(t, wtB, "feat/b", "0544", "task-1", "gate-0544-b", seqFailCmd("acked-current-B"))
		dA, dB := realSeqDriver(store, svc), realSeqDriver(store, svc)
		docA, docB := runPair(t, dA, dB, rA, rB)
		if docA.Outcome != PASSED {
			t.Fatalf("scope A must PASS, got %s (%s)", docA.Outcome, docA.Cause)
		}
		if docB.Outcome != FAILED {
			t.Fatalf("scope B must FAIL (opposite verdict), got %s (%s)", docB.Outcome, docB.Cause)
		}

		// Enumeration by (0543, gate-0543-a) resolves EXACTLY the current drive; the
		// two acknowledged historical drives are terminal-and-consumed and excluded.
		ids, err := store.FindScopeDriveIDs("0543", capHash("gate-0543-a"))
		if err != nil {
			t.Fatalf("FindScopeDriveIDs with history: %v", err)
		}
		if len(ids) != 1 || !idsContain(ids, docA.DriveID) {
			t.Fatalf("enumeration with acked history must resolve exactly the current drive %q, got %v (acked history %v)", docA.DriveID, ids, ackedIDs)
		}
		assertResolvesOwn(t, dA, dB, gA, "0543", "gate-0543-a", docA.DriveID, gB, "0544", "gate-0544-b", docB.DriveID)
	})
}

// TestRaceIntegrationGatedriveSameWorktreeGenerations (spec AC2) drives real generations of
// executions through ONE worktree path over real git and the real process
// supervisor, proving old records for that path never block the next permitted
// drive once release or replacement is proven:
//
//   - drive → release → re-admit across several successive run epochs: a live
//     epoch still owns the worktree between drives (a different epoch is fenced),
//     and once it settles the next epoch admits over its released slot;
//   - a symlink alias of the worktree reaches the SAME slot and admits;
//   - the live-incumbent counter-case: a genuinely executing run on the canonical
//     path refuses a start through the alias, and admits it once the run finishes;
//   - the worktree is removed while its released slot still names the completed
//     epoch: retirement reaches the slot through its stored identity (never
//     re-canonicalization of the missing path), and a recreated worktree at the
//     same path admits a new epoch;
//   - a second remove/recreate leaves the settled epoch on the inherited slot, and
//     the next (epoch-less) start settles it at admission instead of refusing.
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
	epochs := &genSettled{settled: map[string]bool{}}
	d.SetEpochSettledResolver(epochs.resolve)

	slotOf := func(path string) admissionRecord {
		t.Helper()
		slot, _, err := store.LoadWorktreeExecution(path)
		if err != nil {
			t.Fatalf("load slot for %s: %v", path, err)
		}
		return slot
	}
	passAt := func(path, branch, epoch, marker string) DriveDoc {
		t.Helper()
		req := realSeqStart(path, branch, runRoot, "0446", marker, seqPassCmd(marker))
		req.RunEpochID = epoch
		doc := driveSeqToTerminal(t, d, req)
		if doc.Outcome != PASSED {
			t.Fatalf("%s must PASS, got %s (%s)", marker, doc.Outcome, doc.Cause)
		}
		return doc
	}

	// 1. Several successive epochs on one path.
	lastGen := 0
	for i, epoch := range []string{"epoch-g1", "epoch-g2", "epoch-g3"} {
		passAt(wt, "feat/gen", epoch, fmt.Sprintf("gen-%d", i+1))
		slot := slotOf(wt)
		if slot.State != admissionReleased || slot.RunEpochID != epoch {
			t.Fatalf("after %s: slot %s/%q, want released/%s", epoch, slot.State, slot.RunEpochID, epoch)
		}
		if slot.ExecutionGen <= lastGen {
			t.Fatalf("after %s: execution generation %d did not advance past %d", epoch, slot.ExecutionGen, lastGen)
		}
		lastGen = slot.ExecutionGen
		if i == 0 {
			// Unsettled: the live epoch owns its worktree between drives.
			foreign := realSeqStart(wt, "feat/gen", runRoot, "0446", "foreign", seqPassCmd("foreign"))
			foreign.RunEpochID = "epoch-foreign"
			if _, err := d.Start(foreign); !isOwnershipKind(err, ErrStaleRunEpoch) {
				t.Fatalf("an unsettled epoch must fence a different epoch, got %v", err)
			}
		}
		epochs.settle(epoch)
	}

	// 2. A symlink alias reaches the same slot and admits (epoch-less start over the
	// settled epoch-g3's released slot).
	alias := filepath.Join(testsupport.TempDir(t), "wt-alias")
	if err := os.Symlink(wt, alias); err != nil {
		t.Fatalf("symlink alias: %v", err)
	}
	passAt(alias, "feat/gen", "", "gen-alias")
	if slot := slotOf(wt); slot.ExecutionGen != lastGen+1 || slot.RunEpochID != "" {
		t.Fatalf("the alias must reuse the canonical slot: gen %d (want %d) epoch %q", slot.ExecutionGen, lastGen+1, slot.RunEpochID)
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
	if _, err := d.Start(blocked); !isOwnershipKind(err, ErrWorktreeBusy) && !isOwnershipKind(err, ErrUnresolvedExecution) {
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

	// 4. Remove the worktree while its released slot names a completed epoch; retire
	// through the stored identity; recreate the path; a new epoch admits.
	passAt(wt, "feat/gen", "epoch-g4", "gen-4")
	g4 := slotOf(wt)
	git(t, repo, "worktree", "remove", "--force", wt)
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("worktree must be removed, stat err = %v", err)
	}
	removedSlot := slotOf(wt) // stored-identity addressing, not re-canonicalization
	if removedSlot.RunEpochID != "epoch-g4" || removedSlot.ReservationToken != g4.ReservationToken {
		t.Fatalf("the removed worktree's slot must resolve to its stored record, got epoch %q", removedSlot.RunEpochID)
	}
	if err := store.RetireWorktreeExecutionEpoch(wt, "epoch-g4", g4.ReservationToken); err != nil {
		t.Fatalf("retire a removed worktree's epoch through its stored identity: %v", err)
	}
	git(t, repo, "worktree", "add", wt, "-b", "feat/gen-r1")
	passAt(wt, "feat/gen-r1", "epoch-g5", "gen-5")

	// 5. Remove and recreate again WITHOUT retiring: the settled epoch-g5 left on the
	// inherited released slot is settled at admission, never refused.
	epochs.settle("epoch-g5")
	git(t, repo, "worktree", "remove", "--force", wt)
	git(t, repo, "worktree", "add", wt, "-b", "feat/gen-r2")
	passAt(wt, "feat/gen-r2", "", "gen-6")
	if slot := slotOf(wt); slot.RunEpochID != "" || slot.State != admissionReleased {
		t.Fatalf("final slot = %s/%q, want released and epoch-free", slot.State, slot.RunEpochID)
	}
}
