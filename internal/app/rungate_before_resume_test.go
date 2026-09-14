package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// These are the resume-shares-admission tests (change 0375 Task 12): a
// `run.gate-before --resume` shares the change's prior run epoch. An active prior
// run is refused with a safe locator; a cancelling one is pending; a
// confirmed-cancelled one is superseded and reserves EXACTLY ONE replacement
// dispatch (one winner under a concurrent race); a repeat arm observes that
// reservation. None of these reset the change-owned full-suite budget.

// resumeEpochRepo builds a working repo whose corpus shows change 5 in-progress and
// returns the repoDir plus a resume-arming deps/wdeps pair (WorkspaceInspect applies
// at a known worktree). Each call yields independent deps so concurrent resumes share
// only the filesystem under test.
func resumeEpochDeps(t *testing.T) (PlanningDeps, WorkspaceDeps) {
	t.Helper()
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(5, "epsilon", "v5", "")}}
	deps := workspaceDepsFor(t, reader)
	wdeps := WorkspaceDeps{Service: resumeInspectService("/tmp/wt/epsilon")}
	return deps, wdeps
}

// seedPriorEpoch mints a gate record + a run epoch bound to change 5 in the given
// state, returning the prior epoch's gate key and its public epoch id — the run a
// resume of change 5 shares.
func seedPriorEpoch(t *testing.T, repoDir string, state epochState) (gateKey, epochID string) {
	t.Helper()
	key := mintTestGateKey(t, repoDir)
	ep, err := MintEpochRecord(repoDir, key, "5")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	if state != EpochActive {
		if err := epochCAS(repoDir, key, func(r *EpochRecord) error {
			r.State = state
			return nil
		}); err != nil {
			t.Fatalf("epochCAS to %q: %v", state, err)
		}
	}
	return key, ep.EpochID
}

// TestResumeRefusesActiveEpochWithLocator: a resume of a change whose prior epoch is
// still ACTIVE is refused with the safe locator (public epoch id + gate key) and the
// explicit cancel/continue remedy — no record minted, no scope prepared, incumbent
// untouched.
func TestResumeRefusesActiveEpochWithLocator(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	deps, wdeps := resumeEpochDeps(t)
	priorKey, epochID := seedPriorEpoch(t, repoDir, EpochActive)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if res.Armed {
		t.Fatalf("resume armed over an active prior run: %q", res.HumanText())
	}
	if res.Reason != ReasonGateResumeActiveRun {
		t.Fatalf("Reason = %q, want %q", res.Reason, ReasonGateResumeActiveRun)
	}
	if !strings.Contains(res.Message, epochID) || !strings.Contains(res.Message, priorKey) {
		t.Fatalf("Message must name the safe locator (epoch %q, key %q), got %q", epochID, priorKey, res.Message)
	}
	if !strings.Contains(res.Message, "run cancel") {
		t.Fatalf("Message must name the explicit cancel remedy, got %q", res.Message)
	}
	if res.Key != "" || sp.calls != 0 {
		t.Fatalf("active refusal must mint no record and prepare no scope: key=%q calls=%d", res.Key, sp.calls)
	}
	// The incumbent epoch is untouched.
	ep, _, err := LoadEpochRecord(repoDir, priorKey)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.State != EpochActive {
		t.Fatalf("active refusal must not mutate the incumbent epoch, got %q", ep.State)
	}
}

// TestResumeCancellingIsPending: a resume of a change whose prior epoch is CANCELLING
// is refused cancellation-pending — cleanup is still in flight, no replacement.
func TestResumeCancellingIsPending(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	deps, wdeps := resumeEpochDeps(t)
	seedPriorEpoch(t, repoDir, EpochCancelling)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if res.Armed {
		t.Fatalf("resume armed over a cancelling prior run: %q", res.HumanText())
	}
	if res.Reason != ReasonGateResumeCancellationPending {
		t.Fatalf("Reason = %q, want %q", res.Reason, ReasonGateResumeCancellationPending)
	}
	if res.Key != "" || sp.calls != 0 {
		t.Fatalf("cancelling refusal must mint no record and prepare no scope: key=%q calls=%d", res.Key, sp.calls)
	}
}

// TestResumeAfterCancelledSupersedesOnce: two concurrent resumes of a
// confirmed-cancelled run produce EXACTLY ONE winner; the loser observes the
// winner's reservation. The prior epoch ends superseded with the winner's key, and
// exactly one fresh replacement epoch is minted (bound to the feature worktree).
func TestResumeAfterCancelledSupersedesOnce(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	priorKey, _ := seedPriorEpoch(t, repoDir, EpochCancelled)

	deps1, wdeps1 := resumeEpochDeps(t)
	deps2, wdeps2 := resumeEpochDeps(t)
	sp1 := &fakeScopePrep{grant: sampleScopeGrant()}
	sp2 := &fakeScopePrep{grant: sampleScopeGrant()}

	var (
		wg      sync.WaitGroup
		barrier = make(chan struct{})
		res1    RunGateBeforeResult
		res2    RunGateBeforeResult
	)
	wg.Add(2)
	go func() { defer wg.Done(); <-barrier; res1 = RunGateBefore(context.Background(), deps1, wdeps1, sp1.deps(), repoDir, "implement-next", 5) }()
	go func() { defer wg.Done(); <-barrier; res2 = RunGateBefore(context.Background(), deps2, wdeps2, sp2.deps(), repoDir, "implement-next", 5) }()
	close(barrier)
	wg.Wait()

	armed, observed := classifyResumePair(t, res1, res2)
	if armed.Key == "" {
		t.Fatalf("the winner must arm a replacement key")
	}
	if observed.Key != armed.Key {
		t.Fatalf("the loser must observe the winner's reservation: observed %q, winner %q", observed.Key, armed.Key)
	}
	if observed.Reason != ReasonGateResumeReplacementReserved {
		t.Fatalf("loser Reason = %q, want %q", observed.Reason, ReasonGateResumeReplacementReserved)
	}

	// The prior epoch is superseded exactly once, reserving the winner's key.
	prior, _, err := LoadEpochRecord(repoDir, priorKey)
	if err != nil {
		t.Fatalf("LoadEpochRecord(prior): %v", err)
	}
	if prior.State != EpochSuperseded {
		t.Fatalf("prior epoch must be superseded, got %q", prior.State)
	}
	if prior.ReplacementReserved != armed.Key {
		t.Fatalf("prior epoch reserved %q, want the winner's key %q", prior.ReplacementReserved, armed.Key)
	}
	// The winner's replacement epoch is fresh, active, and bound to the feature
	// worktree (so the fence and run.cancel activate for the resumed run).
	repl, _, err := LoadEpochRecord(repoDir, armed.Key)
	if err != nil {
		t.Fatalf("LoadEpochRecord(replacement): %v", err)
	}
	if repl.State != EpochActive {
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
func classifyResumePair(t *testing.T, a, b RunGateBeforeResult) (armed, observed RunGateBeforeResult) {
	t.Helper()
	switch {
	case a.Armed && !b.Armed:
		return a, b
	case b.Armed && !a.Armed:
		return b, a
	default:
		t.Fatalf("exactly one resume must win: a.Armed=%v (%q) b.Armed=%v (%q)", a.Armed, a.HumanText(), b.Armed, b.HumanText())
		return RunGateBeforeResult{}, RunGateBeforeResult{}
	}
}

// TestRepeatArmObservesReservation: after a confirmed-cancelled resume reserves a
// replacement, a SECOND resume of the same change returns that reserved key and
// mints NO new epoch.
func TestRepeatArmObservesReservation(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	seedPriorEpoch(t, repoDir, EpochCancelled)

	deps, wdeps := resumeEpochDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	first := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !first.Armed {
		t.Fatalf("first resume must arm the replacement: %q", first.HumanText())
	}
	epochsAfterFirst := countEpochRecords(t, repoDir)

	deps2, wdeps2 := resumeEpochDeps(t)
	sp2 := &fakeScopePrep{grant: sampleScopeGrant()}
	second := RunGateBefore(context.Background(), deps2, wdeps2, sp2.deps(), repoDir, "implement-next", 5)
	if second.Armed {
		t.Fatalf("a repeat resume must NOT arm a second replacement: %q", second.HumanText())
	}
	if second.Reason != ReasonGateResumeReplacementReserved {
		t.Fatalf("repeat Reason = %q, want %q", second.Reason, ReasonGateResumeReplacementReserved)
	}
	if second.Key != first.Key {
		t.Fatalf("repeat must return the reserved key %q, got %q", first.Key, second.Key)
	}
	if got := countEpochRecords(t, repoDir); got != epochsAfterFirst {
		t.Fatalf("a repeat arm must mint no new epoch: had %d, now %d", epochsAfterFirst, got)
	}
}

// countEpochRecords counts the epoch.json files under the repository's rungate root —
// the number of run epochs, used to prove a repeat arm mints none.
func countEpochRecords(t *testing.T, repoDir string) int {
	t.Helper()
	root, err := gateRoot(repoDir)
	if err != nil {
		t.Fatalf("gateRoot: %v", err)
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
		if _, _, lerr := readStoredEpoch(filepath.Join(root, e.Name()), "count"); lerr == nil {
			n++
		}
	}
	return n
}

// TestResumeDoesNotResetSuiteBudget: a confirmed-cancelled resume that arms a
// replacement never touches the change-owned full-suite attempt budget (spec: an
// explicit human resume "never resets the change-owned full-suite repair budget").
func TestResumeDoesNotResetSuiteBudget(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	seedPriorEpoch(t, repoDir, EpochCancelled)

	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
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

	deps, wdeps := resumeEpochDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	res := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !res.Armed {
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
