// Real-git, real-process integration for a scope that carries a SEQUENCE of
// task-owned drives (change 0405 Task 9, spec verifications "2 (real git)" and 6).
//
// Every other scope-sequence test in this package drives a fakeProc and either a
// fakeGit or a real GitSeam. This file proves the composition a double cannot
// vouch for: the real native process supervisor (internal/process.Service, the
// same seam integration_test.go composes) drives real `/bin/sh -c 'exit N'`
// commands to genuine PASSED/FAILED verdicts, while the real git seam (realGit)
// fingerprints real linked worktrees so a git-visible edit between RED and GREEN
// produces genuinely distinct per-drive fingerprints.
//
// Two properties are load-bearing here and only observable against real git:
//
//   - A recovery scope runs baseline → RED → GREEN as three distinct drives over
//     ONE slot, each successor acknowledging its predecessor receipt, with a real
//     git-visible edit between RED and GREEN; each drive fingerprints the current
//     worktree independently and persists that fingerprint (the successor need NOT
//     match the predecessor — edits are expected).
//
//   - Two scopes in two linked worktrees that share ONE git common dir (hence one
//     drive/scope store) run concurrently with distinguishable commands and
//     opposite verdicts, and each parent's takeover and enumeration resolve ONLY
//     its own scope's current drive — repeated across two tasks of one change, two
//     different changes, and with acknowledged historical drives present in the
//     store — while cross-scope credentials or an explicit old drive id cannot
//     steal work.
//
// The command marker rides in each command's ARGV (as sh's $0), never in a shell
// comment: `exit N` is a shell builtin, so sh never execs it away and the marker
// survives on the sh process's own argv (exec-optimization-erases-the-process-marker).
//
// It reuses this package's real-git fixture helpers (fingerprint_test.go: gitInit,
// writeFile, gitAdd, gitCommit, git) and the real-process fixtures
// (integration_test.go: mustService, mustExe, skipUnlessSupported, reapSupervisors,
// stopAllRuns, advanceUntilTerminal, runDirsUnder), plus the scope helper
// scopeReqFor (takeover_test.go). TestMain (integration_test.go) already routes the
// supervisor re-exec role for the whole package.
package gatedrive

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// ---------------------------------------------------------------------------
// Real-git + real-process fixtures for the sequential-scope integration tests.
// ---------------------------------------------------------------------------

// realSeqDriver wires a driver over the REAL process service and the REAL git
// seam with the injected short slice (integration_test.go's intSlice/intPoll), so
// slices are bounded by real wall-clock time against a live child and fingerprints
// are computed over a real worktree. It is newIntDriver with realGit{} instead of
// the fake stableGit.
func realSeqDriver(store *Store, svc *process.Service) *Driver {
	d := NewDriver(store, systemClock{}, svc, realGit{})
	d.slice = intSlice
	d.pollInterval = intPoll
	d.sleep = time.Sleep
	return d
}

// seedSeqRepo initializes a temporary git repository with one committed file and
// returns its path. Background maintenance is disabled so realGit's own read-only
// queries never spawn a gc/maintenance child that outlives the test (change 0373).
func seedSeqRepo(t *testing.T) string {
	t.Helper()
	repo := testsupport.TempDir(t)
	gitInit(t, repo)
	git(t, repo, "config", "gc.auto", "0")
	git(t, repo, "config", "maintenance.auto", "false")
	writeFile(t, repo, "seed.txt", "seed\n")
	gitAdd(t, repo, "seed.txt")
	gitCommit(t, repo, "seed")
	return repo
}

// addLinkedWorktree adds a linked worktree of repo on a fresh branch and returns
// its absolute path. The linked worktree shares repo's git common dir, so drives
// enrolled under either worktree land in one shared store.
func addLinkedWorktree(t *testing.T, repo, name, branch string) string {
	t.Helper()
	wt := filepath.Join(testsupport.TempDir(t), name)
	git(t, repo, "worktree", "add", wt, "-b", branch)
	return wt
}

// commonDirOf returns the absolute git common directory a worktree resolves to.
// Two linked worktrees of one repository return the identical path — the shared
// common dir the drive/scope store roots under.
func commonDirOf(t *testing.T, worktree string) string {
	t.Helper()
	out := strings.TrimSpace(git(t, worktree, "rev-parse", "--path-format=absolute", "--git-common-dir"))
	abs, err := filepath.Abs(out)
	if err != nil {
		t.Fatalf("abs common dir: %v", err)
	}
	return abs
}

// seqPassCmd / seqFailCmd build distinguishable suite commands whose verdict is a
// real process exit code (0 → PASSED, 1 → FAILED). The marker is a real argv token
// (sh's $0), so it survives exec and is persisted on the drive record's Command.
func seqPassCmd(marker string) []string { return []string{"/bin/sh", "-c", "exit 0", marker} }
func seqFailCmd(marker string) []string { return []string{"/bin/sh", "-c", "exit 1", marker} }

// realSeqStart builds a well-formed StartRequest bound to a real worktree, so its
// fingerprint is computed over real git bytes and its launch runs a real command.
func realSeqStart(worktree, branch, runRoot, changeID, taskID string, cmd []string) StartRequest {
	return StartRequest{
		RepoDir:             worktree,
		Worktree:            worktree,
		ChangeID:            changeID,
		TaskID:              taskID,
		Phase:               "build",
		Branch:              branch,
		Ref:                 "refs/heads/" + branch,
		Command:             cmd,
		Cwd:                 worktree,
		ConfigProvenance:    "config:build.test_command",
		Budget:              30 * time.Minute,
		EnvHash:             "seq-env",
		RunRoot:             runRoot,
		IdempotentSuiteGate: true,
	}
}

// withReceipt returns a copy of req carrying the successor receipt from a
// predecessor's terminal document.
func withReceipt(req StartRequest, pred DriveDoc) StartRequest {
	req.PredecessorDriveID = pred.DriveID
	req.PredecessorOwnerGen = pred.Generation
	return req
}

// driveSeqToTerminal starts req and drives it to a terminal outcome on the test's
// own goroutine (it may call t.Fatalf), asserting every invocation is
// slice-bounded via advanceUntilTerminal.
func driveSeqToTerminal(t *testing.T, d *Driver, req StartRequest) DriveDoc {
	t.Helper()
	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("start %v: %v", req.Command, err)
	}
	if doc.Outcome == WAITING {
		term, _ := advanceUntilTerminal(t, d, doc.DriveID, doc.Generation)
		return term
	}
	return doc
}

// driveSeqToTerminalErr is the goroutine-safe drive-to-terminal: it never touches
// *testing.T (t.Fatalf must be called only from the test goroutine), returning any
// error for the caller to assert after the goroutines join.
func driveSeqToTerminalErr(d *Driver, req StartRequest) (DriveDoc, error) {
	doc, err := d.Start(req)
	if err != nil {
		return DriveDoc{}, err
	}
	deadline := time.Now().Add(30 * time.Second)
	for doc.Outcome == WAITING {
		if time.Now().After(deadline) {
			return doc, fmt.Errorf("drive %s never left WAITING", doc.DriveID)
		}
		doc, err = d.Advance(doc.DriveID, doc.Generation)
		if err != nil {
			return DriveDoc{}, err
		}
	}
	return doc, nil
}

// mustLoad loads a drive record or fails the test.
func mustLoad(t *testing.T, store *Store, id string) driveRecord {
	t.Helper()
	rec, err := store.Load(id)
	if err != nil {
		t.Fatalf("load drive %s: %v", id, err)
	}
	return rec
}

// assertCommandMarker asserts the persisted command carries marker as an argv
// token — proof the distinguishable command was recorded for this drive.
func assertCommandMarker(t *testing.T, rec driveRecord, marker string) {
	t.Helper()
	for _, a := range rec.Command {
		if a == marker {
			return
		}
	}
	t.Fatalf("drive command %v must carry the argv marker %q", rec.Command, marker)
}

// idsContain reports whether ids includes want.
func idsContain(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Verification 2 (real git): baseline → RED → GREEN as a real-git sequence.
// ---------------------------------------------------------------------------

// TestIntegrationSequenceRealGitBaselineRedGreen drives a real recovery-scope
// sequence over real git and the real process supervisor: a PASSED baseline, a
// FAILED RED, a real git-visible edit, then a PASSED GREEN. It proves three
// distinct drives run over one slot with exactly one execution each, each
// fingerprinting the current worktree independently — so the edit between RED and
// GREEN yields a genuinely different persisted GREEN fingerprint — while the scope
// chains the slot and retires each acknowledged predecessor's authority.
func TestIntegrationSequenceRealGitBaselineRedGreen(t *testing.T) {
	skipUnlessSupported(t)
	repo := seedSeqRepo(t)
	wt := addLinkedWorktree(t, repo, "wt", "feat/seq")
	common := commonDirOf(t, wt)
	store := OpenStore(common)
	svc := mustService(t)
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
	reapSupervisors(t, runRoot)
	d := realSeqDriver(store, svc)

	const gateCtx = "seq-real-git-context"
	base := realSeqStart(wt, "feat/seq", runRoot, "0530", "task-9", seqPassCmd("scope-seq-baseline"))
	grant, err := store.PrepareScope(scopeReqFor(base, gateCtx))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	base.ScopeID = grant.ScopeID
	base.ChildCapability = grant.ChildCapability
	base.GateContext = gateCtx

	baseDoc := driveSeqToTerminal(t, d, base)
	if baseDoc.Outcome != PASSED {
		t.Fatalf("baseline over real git+process must PASS, got %s (%s)", baseDoc.Outcome, baseDoc.Cause)
	}

	redReq := withReceipt(base, baseDoc)
	redReq.Command = seqFailCmd("scope-seq-red")
	redDoc := driveSeqToTerminal(t, d, redReq)
	if redDoc.Outcome != FAILED {
		t.Fatalf("RED must FAIL, got %s (%s)", redDoc.Outcome, redDoc.Cause)
	}

	// A real, git-visible edit between RED and GREEN: an untracked file changes the
	// worktree's fingerprint (see TestFingerprintUntrackedFileAdded). Each drive
	// fingerprints the current worktree independently.
	writeFile(t, wt, "between-red-and-green.txt", "edited between RED and GREEN\n")

	greenReq := withReceipt(base, redDoc)
	greenReq.Command = seqPassCmd("scope-seq-green")
	greenDoc := driveSeqToTerminal(t, d, greenReq)
	if greenDoc.Outcome != PASSED {
		t.Fatalf("GREEN must PASS after the edit, got %s (%s)", greenDoc.Outcome, greenDoc.Cause)
	}

	// Three distinct drives, and exactly one raw execution each (no relaunch on a
	// clean terminal): three run directories under the run root.
	ids := map[string]bool{baseDoc.DriveID: true, redDoc.DriveID: true, greenDoc.DriveID: true}
	if len(ids) != 3 {
		t.Fatalf("baseline/RED/GREEN must be three distinct drives, got %v", ids)
	}
	if got := len(runDirsUnder(t, runRoot)); got != 3 {
		t.Fatalf("each of three drives must execute exactly once, got %d raw run dirs", got)
	}

	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.DriveCount != 3 {
		t.Fatalf("scope must have admitted three drives, got DriveCount=%d", scope.DriveCount)
	}
	if scope.CurrentDriveID != greenDoc.DriveID {
		t.Fatalf("current drive must be GREEN %q, got %q", greenDoc.DriveID, scope.CurrentDriveID)
	}
	if scope.PriorDriveID != redDoc.DriveID {
		t.Fatalf("prior drive must be RED %q, got %q", redDoc.DriveID, scope.PriorDriveID)
	}

	baseRec := mustLoad(t, store, baseDoc.DriveID)
	redRec := mustLoad(t, store, redDoc.DriveID)
	greenRec := mustLoad(t, store, greenDoc.DriveID)

	// Per-drive fingerprints persist independently: baseline and RED ran over the
	// same (pre-edit) worktree, so they agree; GREEN ran over the post-edit worktree,
	// so it differs — the real edit is genuinely visible across the git seam.
	if !baseRec.Fingerprint.Equal(redRec.Fingerprint) {
		t.Fatalf("baseline and RED over the same worktree must share a fingerprint")
	}
	if greenRec.Fingerprint.Equal(redRec.Fingerprint) {
		t.Fatalf("the git-visible edit between RED and GREEN must change GREEN's persisted fingerprint")
	}
	live, err := ComputeLiveFingerprint(wt)
	if err != nil {
		t.Fatalf("ComputeLiveFingerprint: %v", err)
	}
	if !live.Equal(greenRec.Fingerprint) {
		t.Fatalf("GREEN's persisted fingerprint must match the current (post-edit) worktree")
	}

	// Distinguishable commands persisted per drive (markers ride in argv).
	assertCommandMarker(t, baseRec, "scope-seq-baseline")
	assertCommandMarker(t, redRec, "scope-seq-red")
	assertCommandMarker(t, greenRec, "scope-seq-green")

	// Acknowledged predecessors are consumed history; only the current drive retains
	// its recovery authority.
	if baseRec.OwnerGeneration != "" || redRec.OwnerGeneration != "" {
		t.Fatalf("acknowledged predecessors must be owner-cleared, got base=%q red=%q", baseRec.OwnerGeneration, redRec.OwnerGeneration)
	}
	if greenRec.OwnerGeneration == "" {
		t.Fatalf("the current (unacknowledged) GREEN drive must retain its owner generation")
	}
}

// ---------------------------------------------------------------------------
// Verification 6: concurrent scopes in linked worktrees over one shared common
// dir; each parent resolves only its own scope's current work.
// ---------------------------------------------------------------------------

// TestIntegrationSequenceConcurrentScopesResolveOwnWork runs two scopes in two
// linked worktrees that share ONE git common dir, concurrently, with
// distinguishable commands and opposite verdicts, and proves each parent's
// takeover and enumeration resolve only its own scope's current drive — repeated
// across two tasks of one change, two different changes, and with acknowledged
// historical drives present in the store. Concurrency is real (two goroutines
// driving the shared store); -race is the oracle for cross-scope interference.
func TestIntegrationSequenceConcurrentScopesResolveOwnWork(t *testing.T) {
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

// makeAckedHistory drives a baseline+successor sequence over a fresh scope and then
// terminally acknowledges it, leaving two owner-cleared (consumed) historical drives
// in the store, and returns their ids. It runs on the test goroutine.
func makeAckedHistory(t *testing.T, d *Driver, store *Store, wt, branch, runRoot, changeID, taskID, gateCtx string) []string {
	t.Helper()
	base := realSeqStart(wt, branch, runRoot, changeID, taskID, seqPassCmd("acked-history-baseline"))
	grant, err := store.PrepareScope(scopeReqFor(base, gateCtx))
	if err != nil {
		t.Fatalf("PrepareScope (acked history): %v", err)
	}
	base.ScopeID = grant.ScopeID
	base.ChildCapability = grant.ChildCapability
	base.GateContext = gateCtx

	baseDoc := driveSeqToTerminal(t, d, base)
	if baseDoc.Outcome != PASSED {
		t.Fatalf("acked-history baseline must PASS, got %s (%s)", baseDoc.Outcome, baseDoc.Cause)
	}
	succReq := withReceipt(base, baseDoc)
	succReq.Command = seqPassCmd("acked-history-successor")
	succDoc := driveSeqToTerminal(t, d, succReq)
	if succDoc.Outcome != PASSED {
		t.Fatalf("acked-history successor must PASS, got %s (%s)", succDoc.Outcome, succDoc.Cause)
	}

	ackDoc, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, succDoc.DriveID, succDoc.Generation)
	if err != nil {
		t.Fatalf("Acknowledge (acked history): %v", err)
	}
	if ackDoc.Outcome != PASSED {
		t.Fatalf("acknowledged final result must report PASSED, got %s (%s)", ackDoc.Outcome, ackDoc.Cause)
	}

	// Both drives are now terminal-and-consumed (owner cleared), and the scope is
	// closed as terminally acknowledged.
	baseRec := mustLoad(t, store, baseDoc.DriveID)
	succRec := mustLoad(t, store, succDoc.DriveID)
	if baseRec.OwnerGeneration != "" || succRec.OwnerGeneration != "" {
		t.Fatalf("acknowledged history must be owner-cleared, got base=%q succ=%q", baseRec.OwnerGeneration, succRec.OwnerGeneration)
	}
	if scope, err := store.LoadScope(grant.ScopeID); err != nil {
		t.Fatalf("LoadScope (acked history): %v", err)
	} else if !scope.Closed || !scope.FinalAcked {
		t.Fatalf("terminally acknowledged scope must be closed+final-acked, got closed=%v final=%v", scope.Closed, scope.FinalAcked)
	}
	return []string{baseDoc.DriveID, succDoc.DriveID}
}

// ---------------------------------------------------------------------------
// Verification 6 (theft tail): cross-scope credentials or an explicit old drive
// id cannot steal another scope's work.
// ---------------------------------------------------------------------------

// TestIntegrationSequenceCredentialTheftRejected proves that, across two scopes in
// two linked worktrees sharing one common dir, scope A's child capability cannot
// authorize a start on scope B (ErrScopeCapabilityMismatch, consuming nothing), and
// scope A's drive id cannot be presented as scope B's takeover target (a HALTED
// stale/identity rejection that supersedes nothing).
func TestIntegrationSequenceCredentialTheftRejected(t *testing.T) {
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
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
	reapSupervisors(t, runRoot)
	dA, dB := realSeqDriver(store, svc), realSeqDriver(store, svc)

	// Scope A with a completed drive.
	reqA := realSeqStart(wtA, "feat/a", runRoot, "0551", "task-1", seqPassCmd("theft-A"))
	grantA, err := store.PrepareScope(scopeReqFor(reqA, "gate-A"))
	if err != nil {
		t.Fatalf("PrepareScope A: %v", err)
	}
	reqA.ScopeID = grantA.ScopeID
	reqA.ChildCapability = grantA.ChildCapability
	reqA.GateContext = "gate-A"
	aDoc := driveSeqToTerminal(t, dA, reqA)
	if aDoc.Outcome != PASSED {
		t.Fatalf("scope A drive must PASS, got %s (%s)", aDoc.Outcome, aDoc.Cause)
	}

	// Scope B, freshly prepared, no drive yet.
	reqB := realSeqStart(wtB, "feat/b", runRoot, "0552", "task-1", seqPassCmd("theft-B"))
	grantB, err := store.PrepareScope(scopeReqFor(reqB, "gate-B"))
	if err != nil {
		t.Fatalf("PrepareScope B: %v", err)
	}
	reqB.ScopeID = grantB.ScopeID
	reqB.ChildCapability = grantB.ChildCapability
	reqB.GateContext = "gate-B"

	// Theft 1: scope A's child capability presented on scope B's start. It is refused
	// ErrScopeCapabilityMismatch before any reservation or launch — scope B admits no
	// drive.
	theft := reqB
	theft.ChildCapability = grantA.ChildCapability
	if _, err := dB.Start(theft); !isOwnershipKind(err, ErrScopeCapabilityMismatch) {
		t.Fatalf("cross-scope child capability must be rejected ErrScopeCapabilityMismatch, got %v", err)
	}
	if scopeB, err := store.LoadScope(grantB.ScopeID); err != nil {
		t.Fatalf("LoadScope B: %v", err)
	} else if scopeB.CurrentDriveID != "" || scopeB.DriveCount != 0 {
		t.Fatalf("a rejected credential-theft start must consume nothing, got current=%q count=%d", scopeB.CurrentDriveID, scopeB.DriveCount)
	}

	// A legitimate scope B start still succeeds afterward: the theft touched nothing.
	bDoc := driveSeqToTerminal(t, dB, reqB)
	if bDoc.Outcome != PASSED {
		t.Fatalf("legitimate scope B start after the rejected theft must PASS, got %s (%s)", bDoc.Outcome, bDoc.Cause)
	}
	if bDoc.DriveID == aDoc.DriveID {
		t.Fatalf("scope B's drive must be its own, not scope A's %q", aDoc.DriveID)
	}

	// Theft 2: scope A's (old) drive id presented as scope B's takeover target. The
	// explicit id cannot bypass scope B's current-drive association — it HALTs
	// (stale-predecessor), supersedes nothing, and leaves scope B open.
	took, err := dB.Takeover(grantB.ScopeID, grantB.ParentCapability, aDoc.DriveID)
	if err != nil {
		t.Fatalf("cross-scope takeover-by-id must not be a command error: %v", err)
	}
	if took.Outcome != HALTED {
		t.Fatalf("cross-scope drive id must HALT, got %s (%s)", took.Outcome, took.Cause)
	}
	if !strings.Contains(took.Cause, string(ErrStalePredecessor)) {
		t.Fatalf("cross-scope takeover HALT cause must name %q, got %q", ErrStalePredecessor, took.Cause)
	}
	// Scope A's drive is untouched: its owner generation was not superseded.
	if aRec := mustLoad(t, store, aDoc.DriveID); aRec.OwnerGeneration != aDoc.Generation {
		t.Fatalf("the stolen drive id's owner must be intact: got %q, want %q", aRec.OwnerGeneration, aDoc.Generation)
	}
	// Scope B stayed open — the rejected takeover claimed nothing.
	if scopeB, err := store.LoadScope(grantB.ScopeID); err != nil {
		t.Fatalf("LoadScope B after theft: %v", err)
	} else if scopeB.Closed {
		t.Fatalf("a rejected cross-scope takeover must not close scope B")
	}
}
