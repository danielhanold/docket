//go:build integration

package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/testsupport"
)

// These are the resume-shares-admission tests (change 0375 Task 12): a
// `run.start --resume` shares the change's prior run epoch. An active prior
// run is refused with a safe locator; a cancelling one is pending; a
// confirmed-cancelled one is superseded and reserves EXACTLY ONE replacement
// dispatch (one winner under a concurrent race); a repeat arm observes that
// reservation. None of these reset the change-owned full-suite budget.

// resumeEpochRepo builds a working repo whose corpus shows change 5 in-progress and
// returns the repoDir plus a resume-arming deps/wdeps pair (WorkspaceInspect applies
// at a known worktree). Each call yields independent deps so concurrent resumes share
// only the filesystem under test.
func resumeRunDeps(t *testing.T) (PlanningDeps, WorkspaceDeps) {
	t.Helper()
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(5, "epsilon", "v5", "")}}
	deps := workspaceDepsFor(t, reader)
	wdeps := WorkspaceDeps{Service: resumeInspectService("/tmp/wt/epsilon")}
	return deps, wdeps
}

// seedPriorRun mints a gate record + a run epoch bound to change 5 in the given
// state, returning the prior epoch's gate key and its public epoch id — the run a
// resume of change 5 shares.
func seedPriorRun(t *testing.T, repoDir string, state runState) (runKey, runID string) {
	t.Helper()
	key := mintTestRunKey(t, repoDir)
	ep, err := MintRunRecord(repoDir, key, "5")
	if err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	if state != RunActive {
		if err := runRecordCAS(repoDir, key, func(r *RunRecord) error {
			r.State = state
			return nil
		}); err != nil {
			t.Fatalf("runRecordCAS to %q: %v", state, err)
		}
	}
	return key, ep.RunID
}

// TestIntegrationRunStartResumeRefusesActiveRunWithLocator: a resume of a change whose prior epoch is
// still ACTIVE is refused with the safe locator (public epoch id + gate key) and the
// explicit cancel/continue remedy — no record minted, no scope prepared, incumbent
// untouched.
func TestIntegrationRunStartResumeRefusesActiveRunWithLocator(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	deps, wdeps := resumeRunDeps(t)
	priorKey, runID := seedPriorRun(t, repoDir, RunActive)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if res.Started {
		t.Fatalf("resume armed over an active prior run: %q", res.HumanText())
	}
	if res.Reason != ReasonRunResumeActiveRun {
		t.Fatalf("Reason = %q, want %q", res.Reason, ReasonRunResumeActiveRun)
	}
	if !strings.Contains(res.Message, runID) || !strings.Contains(res.Message, priorKey) {
		t.Fatalf("Message must name the safe locator (epoch %q, key %q), got %q", runID, priorKey, res.Message)
	}
	if !strings.Contains(res.Message, "run cancel") {
		t.Fatalf("Message must name the explicit cancel remedy, got %q", res.Message)
	}
	if res.Key != "" || sp.calls != 0 {
		t.Fatalf("active refusal must mint no record and prepare no scope: key=%q calls=%d", res.Key, sp.calls)
	}
	// The incumbent epoch is untouched.
	ep, _, err := LoadRunRecord(repoDir, priorKey)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.State != RunActive {
		t.Fatalf("active refusal must not mutate the incumbent epoch, got %q", ep.State)
	}
}

// TestIntegrationRunStartResumeRefusesCompletingRunWithoutSuperseding: a resume of a change whose prior
// epoch is COMPLETING (a verified successful run mid-closeout, change 0441) is refused
// run-untracked with the run-completing reason — it names the keyed run verdict/cancel
// remedy, never turns the closeout into a cancelled predecessor, and reserves no
// replacement.
func TestIntegrationRunStartResumeRefusesCompletingRunWithoutSuperseding(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	deps, wdeps := resumeRunDeps(t)
	priorKey, runID := seedPriorRun(t, repoDir, RunCompleting)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if res.Started {
		t.Fatalf("resume armed over a completing prior run: %q", res.HumanText())
	}
	if res.Reason != ReasonRunResumeRunCompleting {
		t.Fatalf("Reason = %q, want %q", res.Reason, ReasonRunResumeRunCompleting)
	}
	if !strings.Contains(res.Message, runID) || !strings.Contains(res.Message, "run verdict") {
		t.Fatalf("Message must name the epoch %q and the keyed run verdict remedy, got %q", runID, res.Message)
	}
	if res.Key != "" || sp.calls != 0 {
		t.Fatalf("completing refusal must mint no record and prepare no scope: key=%q calls=%d", res.Key, sp.calls)
	}
	ep, _, err := LoadRunRecord(repoDir, priorKey)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.State != RunCompleting {
		t.Fatalf("resume must not mutate the completing epoch, got %q", ep.State)
	}
	if ep.ReplacementReserved != "" {
		t.Fatalf("resume must reserve no replacement over a completing run, got %q", ep.ReplacementReserved)
	}
}

// TestIntegrationRunStartResumeRefusesCompletedRunWithoutSuperseding: a resume of a change whose prior
// epoch is COMPLETED (successful closeout finished, change 0441) is refused run-untracked
// with the run-completed reason — there is nothing to resume; the state and any
// reservation stay untouched (never quiescence-checked into a supersede).
func TestIntegrationRunStartResumeRefusesCompletedRunWithoutSuperseding(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	deps, wdeps := resumeRunDeps(t)
	priorKey, runID := seedPriorRun(t, repoDir, RunCompleted)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if res.Started {
		t.Fatalf("resume armed over a completed prior run: %q", res.HumanText())
	}
	if res.Reason != ReasonRunResumeRunCompleted {
		t.Fatalf("Reason = %q, want %q", res.Reason, ReasonRunResumeRunCompleted)
	}
	if !strings.Contains(res.Message, runID) || !strings.Contains(res.Message, "run verify") {
		t.Fatalf("Message must name the epoch %q and the verify/finalize remedy, got %q", runID, res.Message)
	}
	if res.Key != "" || sp.calls != 0 {
		t.Fatalf("completed refusal must mint no record and prepare no scope: key=%q calls=%d", res.Key, sp.calls)
	}
	ep, _, err := LoadRunRecord(repoDir, priorKey)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.State != RunCompleted {
		t.Fatalf("resume must not mutate the completed epoch, got %q", ep.State)
	}
	if ep.ReplacementReserved != "" {
		t.Fatalf("resume must reserve no replacement over a completed run, got %q", ep.ReplacementReserved)
	}
}

// TestIntegrationRunStartResumeCancellingIsPending: a resume of a change whose prior epoch is CANCELLING
// is refused cancellation-pending — cleanup is still in flight, no replacement.
func TestIntegrationRunStartResumeCancellingIsPending(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	deps, wdeps := resumeRunDeps(t)
	seedPriorRun(t, repoDir, RunCancelling)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if res.Started {
		t.Fatalf("resume armed over a cancelling prior run: %q", res.HumanText())
	}
	if res.Reason != ReasonRunResumeCancellationPending {
		t.Fatalf("Reason = %q, want %q", res.Reason, ReasonRunResumeCancellationPending)
	}
	if res.Key != "" || sp.calls != 0 {
		t.Fatalf("cancelling refusal must mint no record and prepare no scope: key=%q calls=%d", res.Key, sp.calls)
	}
}

// TestRaceIntegrationAppConcurrencyResumeAfterCancelledSupersedesOnce: two concurrent resumes of a
// confirmed-cancelled run produce EXACTLY ONE winner; the loser observes the
// winner's reservation. The prior epoch ends superseded with the winner's key, and
// exactly one fresh replacement epoch is minted (bound to the feature worktree).
// Race shard (change 0465): two RunStart resume arms released by one barrier race to supersede the same cancelled epoch.
func TestRaceIntegrationAppConcurrencyResumeAfterCancelledSupersedesOnce(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	priorKey, _ := seedPriorRun(t, repoDir, RunCancelled)

	deps1, wdeps1 := resumeRunDeps(t)
	deps2, wdeps2 := resumeRunDeps(t)
	sp1 := &fakeScopePrep{grant: sampleScopeGrant()}
	sp2 := &fakeScopePrep{grant: sampleScopeGrant()}

	var (
		wg      sync.WaitGroup
		barrier = make(chan struct{})
		res1    RunStartResult
		res2    RunStartResult
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-barrier
		res1 = RunStart(context.Background(), deps1, wdeps1, sp1.deps(), repoDir, "implement-next", 5)
	}()
	go func() {
		defer wg.Done()
		<-barrier
		res2 = RunStart(context.Background(), deps2, wdeps2, sp2.deps(), repoDir, "implement-next", 5)
	}()
	close(barrier)
	wg.Wait()

	armed, observed := classifyResumePair(t, res1, res2)
	if armed.Key == "" {
		t.Fatalf("the winner must arm a replacement key")
	}
	if observed.Key != armed.Key {
		t.Fatalf("the loser must observe the winner's reservation: observed %q, winner %q", observed.Key, armed.Key)
	}
	if observed.Reason != ReasonRunResumeReplacementReserved {
		t.Fatalf("loser Reason = %q, want %q", observed.Reason, ReasonRunResumeReplacementReserved)
	}

	// The prior epoch is superseded exactly once, reserving the winner's key.
	prior, _, err := LoadRunRecord(repoDir, priorKey)
	if err != nil {
		t.Fatalf("LoadRunRecord(prior): %v", err)
	}
	if prior.State != RunSuperseded {
		t.Fatalf("prior epoch must be superseded, got %q", prior.State)
	}
	if prior.ReplacementReserved != armed.Key {
		t.Fatalf("prior epoch reserved %q, want the winner's key %q", prior.ReplacementReserved, armed.Key)
	}
	// The winner's replacement epoch is fresh, active, and bound to the feature
	// worktree (so the fence and run.cancel activate for the resumed run).
	repl, _, err := LoadRunRecord(repoDir, armed.Key)
	if err != nil {
		t.Fatalf("LoadRunRecord(replacement): %v", err)
	}
	if repl.State != RunActive {
		t.Fatalf("replacement epoch must be active, got %q", repl.State)
	}
	if repl.Worktree != "/tmp/wt/epsilon" {
		t.Fatalf("replacement epoch must bind the feature worktree, got %q", repl.Worktree)
	}
	if repl.ChangeID != "" {
		t.Fatalf("replacement epoch must leave its change unbound until claim, got %q", repl.ChangeID)
	}
}

// classifyResumePair returns (armed, observed) from a resume pair, failing unless
// EXACTLY ONE armed — the one-winner invariant this whole task enforces.
func classifyResumePair(t *testing.T, a, b RunStartResult) (armed, observed RunStartResult) {
	t.Helper()
	switch {
	case a.Started && !b.Started:
		return a, b
	case b.Started && !a.Started:
		return b, a
	default:
		t.Fatalf("exactly one resume must win: a.Armed=%v (%q) b.Armed=%v (%q)", a.Started, a.HumanText(), b.Started, b.HumanText())
		return RunStartResult{}, RunStartResult{}
	}
}

// TestIntegrationRunStartRepeatArmObservesReservation: after a confirmed-cancelled resume reserves a
// replacement, a SECOND resume of the same change returns that reserved key and
// mints NO new epoch.
func TestIntegrationRunStartRepeatArmObservesReservation(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	seedPriorRun(t, repoDir, RunCancelled)

	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	first := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !first.Started {
		t.Fatalf("first resume must arm the replacement: %q", first.HumanText())
	}
	runsAfterFirst := countRunRecords(t, repoDir)

	deps2, wdeps2 := resumeRunDeps(t)
	sp2 := &fakeScopePrep{grant: sampleScopeGrant()}
	second := RunStart(context.Background(), deps2, wdeps2, sp2.deps(), repoDir, "implement-next", 5)
	if second.Started {
		t.Fatalf("a repeat resume must NOT arm a second replacement: %q", second.HumanText())
	}
	if second.Reason != ReasonRunResumeReplacementReserved {
		t.Fatalf("repeat Reason = %q, want %q", second.Reason, ReasonRunResumeReplacementReserved)
	}
	if second.Key != first.Key {
		t.Fatalf("repeat must return the reserved key %q, got %q", first.Key, second.Key)
	}
	if got := countRunRecords(t, repoDir); got != runsAfterFirst {
		t.Fatalf("a repeat arm must mint no new epoch: had %d, now %d", runsAfterFirst, got)
	}
}

// countRunRecords counts the run.json files under the repository's rungate root —
// the number of run epochs, used to prove a repeat arm mints none.
func countRunRecords(t *testing.T, repoDir string) int {
	t.Helper()
	root, err := runTrackerRoot(repoDir)
	if err != nil {
		t.Fatalf("runTrackerRoot: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read rungate root: %v", err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, _, lerr := readStoredRun(filepath.Join(root, e.Name()), "count"); lerr == nil {
			n++
		}
	}
	return n
}

// seedResumeSlot binds the prior epoch's Worktree to a fresh real directory and
// reserves a RELEASED worktree slot there owned by ownerRun, returning the common
// dir, the store, and the bound worktree. The slot-bearing resume-quiescence tests
// (change 0435) use it: the epoch's Worktree must be the same directory the slot is
// reserved for, mirroring the cancel fixture's runRecordCAS worktree bind.
func seedResumeSlot(t *testing.T, repoDir, priorKey, ownerRun string) (common string, store *gatedrive.Store, worktree string) {
	t.Helper()
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	store = gatedrive.OpenStore(common)
	worktree = testsupport.TempDir(t)
	if err := runRecordCAS(repoDir, priorKey, func(r *RunRecord) error {
		r.Worktree = worktree
		return nil
	}); err != nil {
		t.Fatalf("runRecordCAS bind worktree: %v", err)
	}
	token, err := store.ReserveWorktreeExecutionForRun(common, worktree, ownerRun, nil)
	if err != nil {
		t.Fatalf("ReserveWorktreeExecutionForRun: %v", err)
	}
	if err := store.ConfirmWorktreeExecution(worktree, token, "run-1", filepath.Join(worktree, "rd")); err != nil {
		t.Fatalf("ConfirmWorktreeExecution: %v", err)
	}
	if err := store.ReleaseWorktreeExecution(worktree, token); err != nil {
		t.Fatalf("ReleaseWorktreeExecution: %v", err)
	}
	return common, store, worktree
}

// TestIntegrationRunStartResumeDeniedWhileOldRunNotQuiescent (AC6/AC7): a durably cancelled epoch
// whose launch evidence is still unsettled cannot authorize a replacement — the arm
// refuses on the existing run-untracked channel (ReasonRunResumeCancellationPending),
// mints no record, reserves no replacement, and leaves the old epoch cancelled.
func TestIntegrationRunStartResumeDeniedWhileOldRunNotQuiescent(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	priorKey, _ := seedPriorRun(t, repoDir, RunCancelled)

	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	d := sp.deps()
	d.CancelSeams = func(string) cancelSeams {
		return cancelSeams{launches: &fakeLaunchReconciler{report: gatedrive.RunLaunchReport{
			Accounted: false, Findings: []string{"claim-busy:d1"}}}}
	}
	res := RunStart(context.Background(), deps, wdeps, d, repoDir, "implement-next", 5)
	if res.Started {
		t.Fatalf("resume armed over a non-quiescent old epoch: %q", res.HumanText())
	}
	if res.Reason != ReasonRunResumeCancellationPending {
		t.Fatalf("Reason = %q, want %q", res.Reason, ReasonRunResumeCancellationPending)
	}
	if !strings.Contains(res.Message, "claim-busy:d1") {
		t.Fatalf("Message must carry the unresolved finding, got %q", res.Message)
	}
	if res.Key != "" || sp.calls != 0 {
		t.Fatalf("a denied resume must mint no record and prepare no scope: key=%q calls=%d", res.Key, sp.calls)
	}
	prior, _, err := LoadRunRecord(repoDir, priorKey)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if prior.State != RunCancelled {
		t.Fatalf("denied resume must leave the old epoch cancelled, got %q", prior.State)
	}
	if prior.ReplacementReserved != "" {
		t.Fatalf("denied resume must reserve no replacement, got %q", prior.ReplacementReserved)
	}
}

// TestIntegrationRunStartResumeRetiresStaleSlotThenReservesOnce (AC1/AC7): a cancelled epoch whose
// released slot still carries its RunID is retired by the resume validation,
// then EXACTLY ONE replacement is reserved; a repeat arm observes the same key.
func TestIntegrationRunStartResumeRetiresStaleSlotThenReservesOnce(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	priorKey, runID := seedPriorRun(t, repoDir, RunCancelled)
	_, store, worktree := seedResumeSlot(t, repoDir, priorKey, runID)

	mkSeams := func(string) cancelSeams {
		return cancelSeams{store: store, launches: okLaunchReconciler()}
	}

	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	d := sp.deps()
	d.CancelSeams = mkSeams
	first := RunStart(context.Background(), deps, wdeps, d, repoDir, "implement-next", 5)
	if !first.Started {
		t.Fatalf("first resume must arm the replacement: %q", first.HumanText())
	}
	// The stale ownership was retired: RunID cleared, state still released (the
	// replacement's own drive reserves it later).
	if epo := loadSlotRun(t, store, worktree); epo != "" {
		t.Fatalf("slot RunID = %q, want cleared by resume retirement", epo)
	}
	if st := loadSlotState(t, store, worktree); st != "released" {
		t.Fatalf("slot state = %q, want released", st)
	}

	// A repeat arm observes the SAME single reservation — never a second.
	deps2, wdeps2 := resumeRunDeps(t)
	sp2 := &fakeScopePrep{grant: sampleScopeGrant()}
	d2 := sp2.deps()
	d2.CancelSeams = mkSeams
	second := RunStart(context.Background(), deps2, wdeps2, d2, repoDir, "implement-next", 5)
	if second.Started {
		t.Fatalf("repeat arm must not arm a second replacement: %q", second.HumanText())
	}
	if second.Reason != ReasonRunResumeReplacementReserved {
		t.Fatalf("repeat Reason = %q, want %q", second.Reason, ReasonRunResumeReplacementReserved)
	}
	if second.Key != first.Key {
		t.Fatalf("repeat must return the reserved key %q, got %q", first.Key, second.Key)
	}
}

// TestIntegrationRunStartResumeSupersededValidatesBeforeObserve (AC6): re-authorizing a previously
// reserved replacement from a SUPERSEDED epoch also requires quiescence; unsettled
// evidence refuses without touching the reservation.
func TestIntegrationRunStartResumeSupersededValidatesBeforeObserve(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	priorKey, _ := seedPriorRun(t, repoDir, RunCancelled)

	// Winner arm (permissive) supersedes and reserves the one replacement.
	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	first := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !first.Started {
		t.Fatalf("winner arm must reserve the replacement: %q", first.HumanText())
	}
	prior, _, err := LoadRunRecord(repoDir, priorKey)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	reservedBefore := prior.ReplacementReserved
	if prior.State != RunSuperseded || reservedBefore == "" {
		t.Fatalf("winner arm must leave the old epoch superseded with a reservation, got state=%q reserved=%q", prior.State, reservedBefore)
	}

	// Re-arm with non-accounted launch evidence: the reserved replacement cannot be
	// re-authorized until quiescence is re-proved.
	deps2, wdeps2 := resumeRunDeps(t)
	sp2 := &fakeScopePrep{grant: sampleScopeGrant()}
	d2 := sp2.deps()
	d2.CancelSeams = func(string) cancelSeams {
		return cancelSeams{launches: &fakeLaunchReconciler{report: gatedrive.RunLaunchReport{
			Accounted: false, Findings: []string{"relaunch-unresolved:r1"}}}}
	}
	res := RunStart(context.Background(), deps2, wdeps2, d2, repoDir, "implement-next", 5)
	if res.Started {
		t.Fatalf("superseded re-arm must not arm over unresolved evidence: %q", res.HumanText())
	}
	if res.Reason != ReasonRunResumeCancellationPending {
		t.Fatalf("Reason = %q, want %q", res.Reason, ReasonRunResumeCancellationPending)
	}
	after, _, err := LoadRunRecord(repoDir, priorKey)
	if err != nil {
		t.Fatalf("LoadRunRecord(after): %v", err)
	}
	if after.ReplacementReserved != reservedBefore {
		t.Fatalf("re-arm must not alter the reservation: before %q after %q", reservedBefore, after.ReplacementReserved)
	}
}

// TestIntegrationRunStartResumeForeignSlotIsNeutral (AC7): a slot owned by a DIFFERENT epoch neither
// blocks nor is touched by resume — quiescent old-epoch evidence still admits the
// replacement, and the foreign slot is byte-identical after.
func TestIntegrationRunStartResumeForeignSlotIsNeutral(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	priorKey, _ := seedPriorRun(t, repoDir, RunCancelled)
	_, store, worktree := seedResumeSlot(t, repoDir, priorKey, "someone-else")

	beforeSlot, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("load slot before: %v", err)
	}

	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	d := sp.deps()
	d.CancelSeams = func(string) cancelSeams {
		return cancelSeams{store: store, launches: okLaunchReconciler()}
	}
	res := RunStart(context.Background(), deps, wdeps, d, repoDir, "implement-next", 5)
	if !res.Started {
		t.Fatalf("a foreign slot must neither block nor be touched by resume: %q", res.HumanText())
	}
	afterSlot, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("load slot after: %v", err)
	}
	if afterSlot.RunID != "someone-else" {
		t.Fatalf("foreign slot RunID = %q, want someone-else (untouched)", afterSlot.RunID)
	}
	if string(afterSlot.State) != string(beforeSlot.State) {
		t.Fatalf("foreign slot state changed: before %q after %q", beforeSlot.State, afterSlot.State)
	}
}

// TestIntegrationRunStartResumeDoesNotResetSuiteBudget: a confirmed-cancelled resume that arms a
// replacement never touches the change-owned full-suite attempt budget (spec: an
// explicit human resume "never resets the change-owned full-suite repair budget").
func TestIntegrationRunStartResumeDoesNotResetSuiteBudget(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	seedPriorRun(t, repoDir, RunCancelled)

	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	store := gatedrive.OpenStore(common)
	key := gatedrive.SuiteBudgetKey{RepoIdentity: common, ChangeID: "5", Phase: "build"}
	if _, _, err := store.ReserveSuiteAttempt(key, 2); err != nil {
		t.Fatalf("ReserveSuiteAttempt: %v", err)
	}
	usedBefore, limitBefore, err := store.SuiteBudgetUsage(key)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage(before): %v", err)
	}

	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !res.Started {
		t.Fatalf("resume must arm the replacement: %q", res.HumanText())
	}

	usedAfter, limitAfter, err := gatedrive.OpenStore(common).SuiteBudgetUsage(key)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage(after): %v", err)
	}
	if usedAfter != usedBefore || limitAfter != limitBefore {
		t.Fatalf("resume changed the suite budget: before (%d/%d) after (%d/%d)", usedBefore, limitBefore, usedAfter, limitAfter)
	}
}

// armSupersededPrior runs the winner resume (permissive production seams) so the
// prior cancelled epoch is superseded with its Worktree cleared and a replacement
// reserved, then rebinds the replacement epoch to a real worktree directory (the
// resume fixture's inspect path is not a real directory). It returns the prior key,
// the prior epoch id, and the replacement's worktree.
func armSupersededPrior(t *testing.T, repoDir string) (priorKey, priorRun, worktree string) {
	t.Helper()
	priorKey, priorRun = seedPriorRun(t, repoDir, RunCancelled)
	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	first := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !first.Started {
		t.Fatalf("winner arm must reserve the replacement: %q", first.HumanText())
	}
	prior, _, err := LoadRunRecord(repoDir, priorKey)
	if err != nil {
		t.Fatalf("LoadRunRecord(prior): %v", err)
	}
	if prior.State != RunSuperseded || prior.Worktree != "" || prior.ReplacementReserved != first.Key {
		t.Fatalf("prior = state %q worktree %q reserved %q, want superseded/cleared/%q", prior.State, prior.Worktree, prior.ReplacementReserved, first.Key)
	}
	worktree = testsupport.TempDir(t)
	if err := runRecordCAS(repoDir, first.Key, func(r *RunRecord) error { r.Worktree = worktree; return nil }); err != nil {
		t.Fatalf("rebind replacement worktree: %v", err)
	}
	return priorKey, priorRun, worktree
}

// TestIntegrationRunStartResumeSupersededBranchAccountsScopeLinkedDrives (change 0446 AC5): on the
// superseded branch the old epoch's Worktree is empty, which is not proof of
// quiescence. The launch census runs with the PREDECESSOR's epoch id against the
// REPLACEMENT's worktree (threaded through ReplacementReserved), and an unaccounted
// scope-linked launch it reports refuses the re-authorization instead of reporting
// "accounted".
func TestIntegrationRunStartResumeSupersededBranchAccountsScopeLinkedDrives(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	priorKey, priorRun, worktree := armSupersededPrior(t, repoDir)
	reservedBefore := func() string {
		ep, _, err := LoadRunRecord(repoDir, priorKey)
		if err != nil {
			t.Fatalf("LoadRunRecord: %v", err)
		}
		return ep.ReplacementReserved
	}()

	launches := &fakeLaunchReconciler{report: gatedrive.RunLaunchReport{
		Accounted: false, Findings: []string{"launch-pending:d1"}}}
	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	d := sp.deps()
	d.CancelSeams = func(string) cancelSeams { return cancelSeams{launches: launches} }
	res := RunStart(context.Background(), deps, wdeps, d, repoDir, "implement-next", 5)
	if res.Started || res.Reason != ReasonRunResumeCancellationPending {
		t.Fatalf("superseded re-arm over an unaccounted scope-linked launch = armed %v reason %q, want refused %q", res.Started, res.Reason, ReasonRunResumeCancellationPending)
	}
	if !strings.Contains(res.Message, "launch-pending:d1") {
		t.Fatalf("Message must carry the launch finding, got %q", res.Message)
	}
	if len(launches.calls) != 1 || launches.calls[0] != worktree+"|"+priorRun {
		t.Fatalf("census calls = %v, want exactly [%s|%s] (replacement worktree, predecessor epoch)", launches.calls, worktree, priorRun)
	}
	if got := func() string {
		ep, _, err := LoadRunRecord(repoDir, priorKey)
		if err != nil {
			t.Fatalf("LoadRunRecord: %v", err)
		}
		return ep.ReplacementReserved
	}(); got != reservedBefore {
		t.Fatalf("refusal altered the reservation: %q -> %q", reservedBefore, got)
	}
}

// tornResumePrior leaves change 5's cancelled prior epoch superseded by a replacement
// key whose epoch was never minted (neverMinted) or minted but never bound to a
// worktree — the state armResumeReplacement leaves when MintRunRecord or its
// Worktree CAS fails after the supersede. It returns the prior epoch id and the
// replacement key.
func tornResumePrior(t *testing.T, repoDir string, neverMinted bool) (priorKey, priorRun, replKey string) {
	t.Helper()
	priorKey, priorRun = seedPriorRun(t, repoDir, RunCancelled)
	replKey = mintTestRunKey(t, repoDir)
	if err := SupersedeCancelledRun(repoDir, priorKey, replKey); err != nil {
		t.Fatalf("SupersedeCancelledRun: %v", err)
	}
	if !neverMinted {
		if _, err := MintRunRecord(repoDir, replKey, ""); err != nil {
			t.Fatalf("MintRunRecord(replacement): %v", err)
		}
	}
	return priorKey, priorRun, replKey
}

// TestIntegrationRunStartResumeTornReplacementConverges (change 0446 spec "Repeated cancellation,
// completion, and admission after safe reconciliation converge using existing
// operations"): after a torn resume a repeat `run.start --resume` is not a
// permanent dead end. The superseded branch addresses the request's own feature
// worktree (what armResumeReplacement binds), runs the census with the predecessor's
// epoch id there, and observes the single reserved key — repeatedly, minting nothing.
// A corrupt replacement epoch still refuses, naming the unreadable record.
func TestIntegrationRunStartResumeTornReplacementConverges(t *testing.T) {
	for _, tc := range []struct {
		name        string
		neverMinted bool
	}{{"replacement-never-minted", true}, {"replacement-unbound", false}} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := newWorkingRepo(t, nil).invocation
			_, priorRun, replKey := tornResumePrior(t, repoDir, tc.neverMinted)
			runsBefore := countRunRecords(t, repoDir)
			for i := 0; i < 2; i++ {
				launches := okLaunchReconciler()
				deps, wdeps := resumeRunDeps(t)
				sp := &fakeScopePrep{grant: sampleScopeGrant()}
				d := sp.deps()
				d.CancelSeams = func(string) cancelSeams { return cancelSeams{launches: launches} }
				res := RunStart(context.Background(), deps, wdeps, d, repoDir, "implement-next", 5)
				if res.Started || res.Reason != ReasonRunResumeReplacementReserved || res.Key != replKey {
					t.Fatalf("arm %d = armed %v reason %q key %q (%q), want the reserved key %q observed", i, res.Started, res.Reason, res.Key, res.Message, replKey)
				}
				if len(launches.calls) != 1 || launches.calls[0] != "/tmp/wt/epsilon|"+priorRun {
					t.Fatalf("census calls = %v, want exactly [/tmp/wt/epsilon|%s] (request worktree, predecessor epoch)", launches.calls, priorRun)
				}
				if sp.calls != 0 {
					t.Fatalf("an observing arm must prepare no scope, got %d", sp.calls)
				}
			}
			if got := countRunRecords(t, repoDir); got != runsBefore {
				t.Fatalf("observing arms minted epochs: had %d, now %d", runsBefore, got)
			}
		})
	}
	t.Run("corrupt-replacement-refused", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
		_, _, replKey := tornResumePrior(t, repoDir, false)
		dir, err := runKeyDir(repoDir, replKey, "test")
		if err != nil {
			t.Fatalf("runKeyDir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, runRecordFileName), []byte("{not json"), 0o600); err != nil {
			t.Fatalf("corrupt replacement epoch: %v", err)
		}
		deps, wdeps := resumeRunDeps(t)
		sp := &fakeScopePrep{grant: sampleScopeGrant()}
		d := sp.deps()
		d.CancelSeams = func(string) cancelSeams { return cancelSeams{launches: okLaunchReconciler()} }
		res := RunStart(context.Background(), deps, wdeps, d, repoDir, "implement-next", 5)
		if res.Started || res.Reason != ReasonRunResumeCancellationPending || !strings.Contains(res.Message, "replacement-epoch-unreadable:"+replKey) {
			t.Fatalf("result = armed %v reason %q message %q, want cancellation-pending naming replacement-epoch-unreadable:%s", res.Started, res.Reason, res.Message, replKey)
		}
	})
}

// TestIntegrationRunStartResumeSupersededChecksReplacementSlot (change 0446 spec §4): the superseded
// branch's slot check uses the replacement's worktree slot to confirm the
// predecessor's epoch no longer holds it — an unreleased predecessor-owned slot
// refuses, a released one is retired through the shared retirement and then
// observed, and a slot the replacement itself holds is the successor outcome.
func TestIntegrationRunStartResumeSupersededChecksReplacementSlot(t *testing.T) {
	for _, tc := range []struct {
		name      string
		owner     func(priorRun string) string
		release   bool
		wantArmed bool   // true: the reservation is observed
		wantRun   string // "" = retired, "prior" = the predecessor's id kept, else literal
	}{
		{"predecessor-unreleased-refuses", func(p string) string { return p }, false, false, "prior"},
		{"predecessor-released-retired", func(p string) string { return p }, true, true, ""},
		{"replacement-held-neutral", func(string) string { return "replacement-epoch" }, true, true, "replacement-epoch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := newWorkingRepo(t, nil).invocation
			_, priorRun, worktree := armSupersededPrior(t, repoDir)
			common, err := runTrackerGitCommonDir(repoDir)
			if err != nil {
				t.Fatalf("runTrackerGitCommonDir: %v", err)
			}
			store := gatedrive.OpenStore(common)
			token, err := store.ReserveWorktreeExecutionForRun(common, worktree, tc.owner(priorRun), nil)
			if err != nil {
				t.Fatalf("reserve: %v", err)
			}
			if tc.release {
				if err := store.ReleaseWorktreeExecution(worktree, token); err != nil {
					t.Fatalf("release: %v", err)
				}
			}

			deps, wdeps := resumeRunDeps(t)
			sp := &fakeScopePrep{grant: sampleScopeGrant()}
			d := sp.deps()
			d.CancelSeams = func(string) cancelSeams { return cancelSeams{store: store, launches: okLaunchReconciler()} }
			res := RunStart(context.Background(), deps, wdeps, d, repoDir, "implement-next", 5)
			if tc.wantArmed {
				if res.Reason != ReasonRunResumeReplacementReserved {
					t.Fatalf("Reason = %q (%q), want the reservation observed", res.Reason, res.Message)
				}
			} else {
				if res.Reason != ReasonRunResumeCancellationPending || !strings.Contains(res.Message, "slot-not-released") {
					t.Fatalf("result = %q %q, want cancellation-pending naming slot-not-released", res.Reason, res.Message)
				}
			}
			want := tc.wantRun
			if want == "prior" {
				want = priorRun
			}
			if epo := loadSlotRun(t, store, worktree); epo != want {
				t.Fatalf("slot epoch = %q, want %q", epo, want)
			}
		})
	}
}

// TestIntegrationRunStartNoRunRecordResumeMintsBoundRun (change 0463): resuming an in-progress change
// that has NO prior run epoch (its first dispatch was never armed) mints one. The
// epoch is bound to the change and to the verified feature worktree, and the result
// carries its id, so the armed line is always `run-started <key> <epoch> <dispatch-context>`.
func TestIntegrationRunStartNoRunRecordResumeMintsBoundRun(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	if _, _, found, err := FindRunByChange(repoDir, "5"); err != nil || found {
		t.Fatalf("fixture must start epochless: found=%v err=%v", found, err)
	}

	res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !res.Started || res.Key == "" {
		t.Fatalf("an epochless resume must arm: %q", res.HumanText())
	}
	if res.RunID == "" {
		t.Fatalf("an epochless resume armed with no epoch: %q", res.HumanText())
	}
	ep, _, err := LoadRunRecord(repoDir, res.Key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.RunID != res.RunID {
		t.Errorf("result Epoch = %q, want the minted epoch id %q", res.RunID, ep.RunID)
	}
	if ep.ChangeID != "5" {
		t.Errorf("epoch ChangeID = %q, want \"5\" (bound to the resumed change)", ep.ChangeID)
	}
	if ep.State != RunActive {
		t.Errorf("epoch state = %q, want active", ep.State)
	}
	if ep.Worktree != "/tmp/wt/epsilon" {
		t.Errorf("epoch Worktree = %q, want the verified feature worktree /tmp/wt/epsilon", ep.Worktree)
	}
	// JSON consumers see the epoch on this path too, and the shape is unchanged.
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"run_id":"` + res.RunID + `"`, `"key":"` + res.Key + `"`, `"dispatch_context":"` + scopeGrantChild + `"`} {
		if !strings.Contains(string(buf), want) {
			t.Errorf("JSON result missing %s: %s", want, buf)
		}
	}
}

// TestIntegrationRunStartRepeatNoRunRecordResumeRefusedActive (change 0463): after an epochless resume
// mints its epoch, a SECOND resume of the same change finds that epoch active and
// refuses resume-active-run with the safe locator, the same single-live-run
// protection every other epoch gets. It mints nothing and prepares no scope.
func TestIntegrationRunStartRepeatNoRunRecordResumeRefusedActive(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	first := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !first.Started {
		t.Fatalf("first epochless resume must arm: %q", first.HumanText())
	}

	deps2, wdeps2 := resumeRunDeps(t)
	sp2 := &fakeScopePrep{grant: sampleScopeGrant()}
	second := RunStart(context.Background(), deps2, wdeps2, sp2.deps(), repoDir, "implement-next", 5)
	if second.Started {
		t.Fatalf("a repeat resume over a live minted epoch must not arm: %q", second.HumanText())
	}
	if second.Reason != ReasonRunResumeActiveRun {
		t.Fatalf("Reason = %q, want %q", second.Reason, ReasonRunResumeActiveRun)
	}
	if !strings.Contains(second.Message, first.RunID) || !strings.Contains(second.Message, first.Key) {
		t.Fatalf("locator must name epoch %q and key %q, got %q", first.RunID, first.Key, second.Message)
	}
	if second.Key != "" || sp2.calls != 0 {
		t.Fatalf("an active refusal mints nothing: key=%q calls=%d", second.Key, sp2.calls)
	}
}

// TestIntegrationRunStartNoRunRecordResumeRunJoinsCancelCycle (change 0463, Review Focus 2): the epoch
// an epochless resume mints goes through the ordinary lifecycle, driven by the REAL
// cancel path. The resume-active-run refusal names `run cancel` as its remedy, so
// that remedy must work with the arm's own key and epoch: a resume arm has no claim
// binding (change.claim requires a proposed change), and runCancel must accept its
// resume-verified attribution instead of refusing claim-unconfirmed forever. Once
// cancelled, the next resume supersedes the epoch and reserves exactly one
// replacement with a fresh epoch, and that replacement is cancellable the same way.
func TestIntegrationRunStartNoRunRecordResumeRunJoinsCancelCycle(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	seams := cancelSeams{store: gatedrive.OpenStore(common), stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}
	mkSeams := func(string) cancelSeams { return seams }

	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	d := sp.deps()
	d.CancelSeams = mkSeams
	first := RunStart(context.Background(), deps, wdeps, d, repoDir, "implement-next", 5)
	if !first.Started {
		t.Fatalf("first epochless resume must arm: %q", first.HumanText())
	}

	// The documented remedy: cancel with the arm's own key and epoch.
	cancel := runCancel(seams, repoDir, first.Key, first.RunID, "human stop")
	if cancel.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancelling the epochless resume's epoch: disposition = %q, want cancelled (findings=%v)", cancel.Disposition, cancel.Findings)
	}
	if st := loadRunState(t, repoDir, first.Key); st != RunCancelled {
		t.Fatalf("epoch state after cancel = %q, want cancelled", st)
	}

	deps2, wdeps2 := resumeRunDeps(t)
	sp2 := &fakeScopePrep{grant: sampleScopeGrant()}
	d2 := sp2.deps()
	d2.CancelSeams = mkSeams
	repl := RunStart(context.Background(), deps2, wdeps2, d2, repoDir, "implement-next", 5)
	if !repl.Started || repl.RunID == "" || repl.RunID == first.RunID {
		t.Fatalf("the resume after cancel must arm one replacement with a fresh epoch: %q (first epoch %q)", repl.HumanText(), first.RunID)
	}
	prior, _, err := LoadRunRecord(repoDir, first.Key)
	if err != nil {
		t.Fatalf("LoadRunRecord(prior): %v", err)
	}
	if prior.State != RunSuperseded || prior.ReplacementReserved != repl.Key {
		t.Fatalf("prior epoch = (%q, reserved %q), want superseded reserving %q", prior.State, prior.ReplacementReserved, repl.Key)
	}

	// The replacement's epoch is armed by resume too, so it is cancellable the same way.
	replCancel := runCancel(seams, repoDir, repl.Key, repl.RunID, "human stop")
	if replCancel.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancelling the replacement epoch: disposition = %q, want cancelled (findings=%v)", replCancel.Disposition, replCancel.Findings)
	}
}

// TestIntegrationRunCancelRunCancelResumeAuthorityFailsClosed (change 0463): the resume-verified
// authority runCancel accepts is narrow. A resume-shaped record whose epoch names a
// DIFFERENT change refuses claim-mismatch, and a record carrying an unconfirmed claim
// reservation refuses claim-unconfirmed even though its AttributedID is set. Neither
// refusal fences the epoch.
func TestIntegrationRunCancelRunCancelResumeAuthorityFailsClosed(t *testing.T) {
	t.Run("epoch names another change", func(t *testing.T) {
		repo := newRunTrackerRepo(t)
		common, _ := runTrackerGitCommonDir(repo)
		key, err := MintRunTrackerRecord(repo, RunTrackerRecord{
			Target: runStartStoredTarget, AttemptLimit: 2, Retry: RetryUnused,
			Disposition: "run-started", ParentCap: "parent-cap-raw", AttributedID: 5,
		})
		if err != nil {
			t.Fatalf("MintRunTrackerRecord: %v", err)
		}
		ep, err := MintRunRecord(repo, key, "6")
		if err != nil {
			t.Fatalf("MintRunRecord: %v", err)
		}
		res := runCancel(cancelSeams{store: gatedrive.OpenStore(common), stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}, repo, key, ep.RunID, "human stop")
		if res.Disposition != CancelDispositionRefused || !hasFinding(res.Findings, "claim-mismatch") {
			t.Fatalf("got (%q, %v), want refused claim-mismatch", res.Disposition, res.Findings)
		}
		if st := loadRunState(t, repo, key); st != RunActive {
			t.Fatalf("a refused cancel must not fence: epoch state = %q", st)
		}
	})
	t.Run("unconfirmed reservation present", func(t *testing.T) {
		repo := newRunTrackerRepo(t)
		common, _ := runTrackerGitCommonDir(repo)
		key, err := MintRunTrackerRecord(repo, RunTrackerRecord{
			Target: runStartStoredTarget, AttemptLimit: 2, Retry: RetryUnused,
			Disposition: "run-started", ParentCap: "parent-cap-raw", AttributedID: 5,
		})
		if err != nil {
			t.Fatalf("MintRunTrackerRecord: %v", err)
		}
		ep, err := MintRunRecord(repo, key, "5")
		if err != nil {
			t.Fatalf("MintRunRecord: %v", err)
		}
		if err := ReserveRunTrackerClaim(repo, key, 5, "req-1"); err != nil {
			t.Fatalf("ReserveRunTrackerClaim: %v", err)
		}
		res := runCancel(cancelSeams{store: gatedrive.OpenStore(common), stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}, repo, key, ep.RunID, "human stop")
		if res.Disposition != CancelDispositionRefused || !hasFinding(res.Findings, "claim-unconfirmed") {
			t.Fatalf("got (%q, %v), want refused claim-unconfirmed", res.Disposition, res.Findings)
		}
		if st := loadRunState(t, repo, key); st != RunActive {
			t.Fatalf("a refused cancel must not fence: epoch state = %q", st)
		}
	})
}

// TestIntegrationRunStartConcurrentNoRunRecordResumeRunsFailSafe (change 0463 decision 4): the
// per-change resume lock keeps concurrent arms from minting two live epochs for one
// change (TestRaceIntegrationAppConcurrencyNoRunRecordResumesArmOnce), but such a pair can still exist,
// for example left by a binary that predates the lock. A resume over it must fail
// closed as resume-run-record-unreadable and must never arm a third run. Recovery is an
// explicit 'docket run cancel' of either epoch by its own key and epoch, which
// runCancel's resume-verified authority accepts (TestIntegrationRunStartNoRunRecordResumeRunJoinsCancelCycle
// drives that cancel path); the surviving epoch is then the worktree's sole live owner.
func TestIntegrationRunStartConcurrentNoRunRecordResumeRunsFailSafe(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	seedPriorRun(t, repoDir, RunActive)
	seedPriorRun(t, repoDir, RunActive)
	deps, wdeps := resumeRunDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if res.Started {
		t.Fatalf("an ambiguous pair of live epochs must never arm: %q", res.HumanText())
	}
	if res.Reason != ReasonResumeRunRecordUnreadable {
		t.Fatalf("Reason = %q, want %q", res.Reason, ReasonResumeRunRecordUnreadable)
	}
	if res.Key != "" || sp.calls != 0 {
		t.Fatalf("a fail-closed refusal mints nothing: key=%q calls=%d", res.Key, sp.calls)
	}
}

// TestRaceIntegrationAppConcurrencyNoRunRecordResumesArmOnce (change 0463, post-review): epochless resume
// arms of one change race from the "no prior epoch" check to the mint and bind. The
// per-change resume lock serializes that window, so exactly one arm wins; every other
// arm then sees the winner's live epoch and refuses resume-active-run. Exactly one
// live epoch may end up bound to the change.
func TestRaceIntegrationAppConcurrencyNoRunRecordResumesArmOnce(t *testing.T) {
	const arms = 12
	repoDir := newWorkingRepo(t, nil).invocation
	results := make([]RunStartResult, arms)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < arms; i++ {
		deps, wdeps := resumeRunDeps(t)
		sp := &fakeScopePrep{grant: sampleScopeGrant()}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
		}(i)
	}
	close(start)
	wg.Wait()

	armed := 0
	for _, r := range results {
		switch {
		case r.Started:
			armed++
		case r.Reason != ReasonRunResumeActiveRun:
			t.Errorf("a losing arm must refuse %q, got %q (%s)", ReasonRunResumeActiveRun, r.Reason, r.Message)
		}
	}
	if armed != 1 {
		t.Fatalf("armed = %d of %d concurrent epochless resumes, want exactly 1", armed, arms)
	}
	if _, _, found, err := FindRunByChange(repoDir, "5"); err != nil || !found {
		t.Fatalf("FindRunByChange after the race: found=%v err=%v, want the single winner's epoch", found, err)
	}
}

// TestIntegrationRunStartResumeRefusalNamesAbandonedArmRemedy (change 0463, post-review): an epochless
// resume arm binds its epoch at arm time, so an arm that was never dispatched blocks
// the next resume until it is cancelled. Nothing records whether an agent is using
// the epoch, so the refusal cannot say which case applies; it names both remedies,
// including the abandoned-arm case, for either kind of incumbent (found by change or
// by worktree).
func TestIntegrationRunStartResumeRefusalNamesAbandonedArmRemedy(t *testing.T) {
	for _, msg := range []string{
		resumeActiveLocator("k", RunRecord{ChangeID: "5", RunID: "e"}),
		resumeWorktreeOwnerLocator("/tmp/wt/epsilon", "k", RunRecord{RunID: "e"}),
	} {
		for _, want := range []string{"never dispatched", "run cancel --key k --run-id e", "still running", "run verdict"} {
			if !strings.Contains(msg, want) {
				t.Errorf("refusal must contain %q, got %q", want, msg)
			}
		}
	}
}

// TestIntegrationRunStartNoRunRecordResumeRefusesLiveWorktreeOwner (change 0463 decision 4, review fix):
// an epochless resume binds its fresh epoch to the verified worktree, so it must not
// mint over a live epoch that already owns that worktree under no change or another
// change. FindRunByChange cannot see such an owner. Minting anyway would leave two
// active epochs on one worktree, and findRunByWorktree would then refuse every
// fenced mutation there as ErrRunOwnerAmbiguous. The resume refuses
// resume-active-run with the owner's locator instead and mints nothing. A fenced
// (cancelled) epoch on the worktree is not a live owner and does not block.
func TestIntegrationRunStartNoRunRecordResumeRefusesLiveWorktreeOwner(t *testing.T) {
	// seedWorktreeOwner mints an epoch bound to worktree (and to changeID, when set)
	// in the given state, returning its gate key and epoch id.
	seedWorktreeOwner := func(t *testing.T, repoDir, changeID, worktree string, state runState) (string, string) {
		t.Helper()
		key := mintTestRunKey(t, repoDir)
		ep, err := MintRunRecord(repoDir, key, changeID)
		if err != nil {
			t.Fatalf("MintRunRecord: %v", err)
		}
		if err := runRecordCAS(repoDir, key, func(r *RunRecord) error {
			r.Worktree = worktree
			r.State = state
			return nil
		}); err != nil {
			t.Fatalf("runRecordCAS bind: %v", err)
		}
		return key, ep.RunID
	}
	assertRefused := func(t *testing.T, res RunStartResult, sp *fakeScopePrep, ownerKey, ownerRun string) {
		t.Helper()
		if res.Started {
			t.Fatalf("resume armed over a live worktree owner: %q", res.HumanText())
		}
		if res.Reason != ReasonRunResumeActiveRun {
			t.Fatalf("Reason = %q, want %q", res.Reason, ReasonRunResumeActiveRun)
		}
		if !strings.Contains(res.Message, ownerRun) || !strings.Contains(res.Message, ownerKey) || !strings.Contains(res.Message, "run cancel") {
			t.Fatalf("Message must name the owner's locator (epoch %q, key %q) and the cancel remedy, got %q", ownerRun, ownerKey, res.Message)
		}
		if res.Key != "" || sp.calls != 0 {
			t.Fatalf("the refusal must mint no record and prepare no scope: key=%q calls=%d", res.Key, sp.calls)
		}
	}

	for _, tc := range []struct {
		name, changeID string
		state          runState
	}{
		{"active owner naming no change", "", RunActive},
		{"active owner naming another change", "7", RunActive},
		{"completing owner naming another change", "7", RunCompleting},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := newWorkingRepo(t, nil).invocation
			ownerKey, ownerRun := seedWorktreeOwner(t, repoDir, tc.changeID, "/tmp/wt/epsilon", tc.state)
			deps, wdeps := resumeRunDeps(t)
			sp := &fakeScopePrep{grant: sampleScopeGrant()}
			res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
			assertRefused(t, res, sp, ownerKey, ownerRun)
			if st := loadRunState(t, repoDir, ownerKey); st != tc.state {
				t.Fatalf("the incumbent must be untouched: state = %q, want %q", st, tc.state)
			}
		})
	}

	t.Run("owner bound under a different spelling of the worktree", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
		raw := filepath.Join(testsupport.TempDir(t), "wt")
		if err := os.MkdirAll(raw, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		link := filepath.Join(testsupport.TempDir(t), "wt-link")
		if err := os.Symlink(raw, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		ownerKey, ownerRun := seedWorktreeOwner(t, repoDir, "", raw, RunActive)
		reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(5, "epsilon", "v5", "")}}
		deps := workspaceDepsFor(t, reader)
		wdeps := WorkspaceDeps{Service: resumeInspectService(link)}
		sp := &fakeScopePrep{grant: sampleScopeGrant()}
		res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
		assertRefused(t, res, sp, ownerKey, ownerRun)
	})

	t.Run("cancelled epoch on the worktree does not block", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
		seedWorktreeOwner(t, repoDir, "7", "/tmp/wt/epsilon", RunCancelled)
		deps, wdeps := resumeRunDeps(t)
		sp := &fakeScopePrep{grant: sampleScopeGrant()}
		res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
		if !res.Started || res.RunID == "" {
			t.Fatalf("a fenced epoch on the worktree is not a live owner; the resume must arm: %q", res.HumanText())
		}
	})
}

// TestIntegrationRunStartArmedGateResultRequiresRun (change 0463): the armed constructor refuses to
// arm without an epoch. That guarantee is what makes the positional three-token
// line unambiguous.
func TestIntegrationRunStartArmedGateResultRequiresRun(t *testing.T) {
	if got := startedRunResult("k", "", "ctx"); got.Started || got.Reason != ReasonRunMintFailed || got.Key != "" || got.RunContext != "" {
		t.Fatalf("an epochless armed result must fail closed as run-untracked mint-failed, got %+v", got)
	}
	got := startedRunResult("k", "e", "ctx")
	if !got.Started || got.Result != ResultApplied || got.Key != "k" || got.RunID != "e" || got.RunContext != "ctx" ||
		got.Target != runStartStoredTarget || got.OwnerLifecycle != ReasonOwnerLifecycleUnavailable {
		t.Fatalf("armed result fields wrong: %+v", got)
	}
}
