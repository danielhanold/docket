package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// These are the `run cancel` operation tests (change 0375 Task 10). run.cancel is
// the coordinator's explicit Stop: it durably fences the run epoch, tears down
// registered tasks/processes, reconciles admitted mutations, and reports cancelled
// only on full accounting. The tests drive the flow over faked stop/native seams and
// a real gatedrive admission store rooted at a real temp git repo.

// fakeCancelStopper is an injectable cancelStopper: it records every run dir it was
// asked to stop, answers proven/unproven from a per-dir map, and (via onStop) lets a
// test inject a race between the fence and the stop.
type fakeCancelStopper struct {
	proven map[string]bool
	calls  []string
	onStop func(runDir string)
}

func (f *fakeCancelStopper) stopProcess(runDir string) (bool, error) {
	f.calls = append(f.calls, runDir)
	if f.onStop != nil {
		f.onStop(runDir)
	}
	return f.proven[runDir], nil
}

// fakeNativeCanceller records the native handles it was asked to cancel and returns
// a canned error.
type fakeNativeCanceller struct {
	calls []string
	err   error
}

func (f *fakeNativeCanceller) cancelNativeTask(handle string) error {
	f.calls = append(f.calls, handle)
	return f.err
}

// fakeLaunchReconciler is an injectable epochLaunchReconciler: it records each
// (worktree,epoch) pair it was asked to reconcile and returns a canned report/error.
type fakeLaunchReconciler struct {
	report gatedrive.EpochLaunchReport
	err    error
	calls  []string
}

func (f *fakeLaunchReconciler) reconcile(worktree, epochID string) (gatedrive.EpochLaunchReport, error) {
	f.calls = append(f.calls, worktree+"|"+epochID)
	return f.report, f.err
}

// okLaunchReconciler is a permissive fake reconciler: every epoch's launch
// obligations are already accounted with no findings, so a cancel test that does not
// exercise the launch-reconciliation path behaves exactly as before the seam existed.
func okLaunchReconciler() *fakeLaunchReconciler {
	return &fakeLaunchReconciler{report: gatedrive.EpochLaunchReport{Accounted: true}}
}

// cancelFixture is one prepared cancelable run: a gate record with a parent-held
// authority, an active epoch bound to change 42 with a confirmed claim, a canonical
// feature worktree, and a confirmed worktree execution slot whose process is runDir.
type cancelFixture struct {
	repo     string
	key      string
	epochID  string
	worktree string
	runDir   string
	store    *gatedrive.Store
	common   string
}

// newCancelFixture builds a fully authorized cancelable run. slot controls whether a
// worktree execution slot is reserved+confirmed; a run with no slot exercises the
// keyless/standalone path.
func newCancelFixture(t *testing.T, slot bool) cancelFixture {
	t.Helper()
	repo := newGateRepo(t)
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	key, err := MintGateRecord(repo, GateRecord{
		Target:       gateBeforeStoredTarget,
		AttemptLimit: 2,
		Retry:        RetryUnused,
		Disposition:  "gate-armed",
		ParentCap:    "parent-cap-raw",
		ScopeID:      "scope-1",
	})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	ep, err := MintEpochRecord(repo, key, "42")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	if err := ReserveGateClaim(repo, key, 42, "req-1"); err != nil {
		t.Fatalf("ReserveGateClaim: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 42, "req-1", "rev-1", ""); err != nil {
		t.Fatalf("ConfirmGateClaim: %v", err)
	}

	worktree := filepath.Join(repo, "feature-wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := epochCAS(repo, key, func(r *EpochRecord) error {
		r.Worktree = worktree
		return nil
	}); err != nil {
		t.Fatalf("epochCAS set worktree: %v", err)
	}

	fx := cancelFixture{repo: repo, key: key, epochID: ep.EpochID, worktree: worktree, common: common}
	fx.store = gatedrive.OpenStore(common)
	if slot {
		runDir := filepath.Join(worktree, "run-1")
		token, terr := fx.store.ReserveRawWorktreeExecution(common, worktree, nil)
		if terr != nil {
			t.Fatalf("ReserveRawWorktreeExecution: %v", terr)
		}
		if cerr := fx.store.ConfirmWorktreeExecution(worktree, token, "run-1", runDir); cerr != nil {
			t.Fatalf("ConfirmWorktreeExecution: %v", cerr)
		}
		fx.runDir = runDir
	}
	return fx
}

// loadEpochState reads the epoch's current state.
func loadEpochState(t *testing.T, repo, key string) epochState {
	t.Helper()
	ep, _, err := LoadEpochRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	return ep.State
}

// loadSlotState reads the worktree slot's current state as a string.
func loadSlotState(t *testing.T, store *gatedrive.Store, worktree string) string {
	t.Helper()
	slot, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	return string(slot.State)
}

func hasFinding(findings []string, prefix string) bool {
	for _, f := range findings {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

// TestRunCancelHappyPath: an active epoch with a proven slot teardown cancels
// cleanly — disposition cancelled, epoch cancelled, slot released.
func TestRunCancelHappyPath(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelled {
		t.Fatalf("epoch state = %q, want cancelled", st)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "released" {
		t.Fatalf("slot state = %q, want released", st)
	}
	if len(stopper.calls) != 1 || stopper.calls[0] != fx.runDir {
		t.Fatalf("stopper calls = %v, want [%s]", stopper.calls, fx.runDir)
	}
}

// TestRunCancelPendingOnUnprovenStop: an unproven slot teardown fences the epoch but
// leaves it cancelling — disposition cancellation-pending, slot stopping.
func TestRunCancelPendingOnUnprovenStop(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: false}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending", res.Disposition)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling (durable fence held)", st)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "stopping" {
		t.Fatalf("slot state = %q, want stopping (not released)", st)
	}
	if !hasFinding(res.Findings, "slot-stop-unproven") {
		t.Fatalf("findings = %v, want a slot-stop-unproven finding", res.Findings)
	}
}

// TestRunCancelAlreadyCancelled: a repeat against a cancelled epoch is idempotent
// already-cancelled, touching nothing.
func TestRunCancelAlreadyCancelled(t *testing.T) {
	fx := newCancelFixture(t, false)
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.State = EpochCancelled
		return nil
	}); err != nil {
		t.Fatalf("epochCAS: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionAlreadyCancelled {
		t.Fatalf("disposition = %q, want already-cancelled", res.Disposition)
	}
	if res.Result != ResultNoOp {
		t.Fatalf("result = %q, want no-op", res.Result)
	}
	if len(stopper.calls) != 0 {
		t.Fatalf("already-cancelled must stop nothing, got %v", stopper.calls)
	}
}

// TestRunCancelRefusedWrongEpoch: a stale epoch locator is refused with no fence.
func TestRunCancelRefusedWrongEpoch(t *testing.T) {
	fx := newCancelFixture(t, true)
	res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}}, fx.repo, fx.key, "not-the-epoch", "human stop")
	if res.Disposition != CancelDispositionRefused {
		t.Fatalf("disposition = %q, want refused", res.Disposition)
	}
	if !hasFinding(res.Findings, "epoch-mismatch") {
		t.Fatalf("findings = %v, want epoch-mismatch", res.Findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochActive {
		t.Fatalf("a refused cancel must not fence: epoch state = %q, want active", st)
	}
}

// TestRunCancelRefusedWrongClaim: an unconfirmed (here, absent) claim binding is
// refused — the run is not a genuinely claimed run.
func TestRunCancelRefusedWrongClaim(t *testing.T) {
	// Build a fixture WITHOUT confirming a claim.
	repo := newGateRepo(t)
	common, _ := gateGitCommonDir(repo)
	key, err := MintGateRecord(repo, GateRecord{
		Target: gateBeforeStoredTarget, AttemptLimit: 2, Retry: RetryUnused,
		Disposition: "gate-armed", ParentCap: "parent-cap-raw",
	})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	ep, err := MintEpochRecord(repo, key, "42")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	res := runCancel(cancelSeams{store: gatedrive.OpenStore(common), stopper: &fakeCancelStopper{}}, repo, key, ep.EpochID, "human stop")
	if res.Disposition != CancelDispositionRefused {
		t.Fatalf("disposition = %q, want refused", res.Disposition)
	}
	if !hasFinding(res.Findings, "claim-unconfirmed") {
		t.Fatalf("findings = %v, want claim-unconfirmed", res.Findings)
	}
	if st := loadEpochState(t, repo, key); st != EpochActive {
		t.Fatalf("a refused cancel must not fence: epoch state = %q, want active", st)
	}
}

// TestRunCancelRefusedWrongRepo: a key that does not locate a record in this
// repository is refused (the repository/locator authority fails closed).
func TestRunCancelRefusedWrongRepo(t *testing.T) {
	fx := newCancelFixture(t, false)
	other := newGateRepo(t)
	otherCommon, _ := gateGitCommonDir(other)
	res := runCancel(cancelSeams{store: gatedrive.OpenStore(otherCommon), stopper: &fakeCancelStopper{}}, other, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionRefused {
		t.Fatalf("disposition = %q, want refused (findings=%v)", res.Disposition, res.Findings)
	}
	// The original epoch in the real repo is untouched.
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochActive {
		t.Fatalf("a foreign-repo cancel must not touch the real epoch: state = %q", st)
	}
}

// TestCancelFencesBeforeStopping: a participant that registers between the fence and
// the stop (a launch admitted before the fence won) is caught by the post-stop
// re-enumeration, keeping the cancellation pending.
func TestCancelFencesBeforeStopping(t *testing.T) {
	fx := newCancelFixture(t, true)
	// A raw-run participant present at entry; stopping it proves teardown.
	if err := RegisterEpochParticipant(fx.repo, fx.key, fx.epochID, EpochParticipant{Kind: "raw-run", NativeHandle: "R1"}); err != nil {
		t.Fatalf("RegisterEpochParticipant P1: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{"R1": true, fx.runDir: true}}
	// The barrier: when P1 is stopped (after the fence), a racing launch registers
	// P2 directly (the epoch is cancelling, so the normal registration path would be
	// refused — this stands in for a launch that reserved its slot before the fence).
	stopper.onStop = func(runDir string) {
		if runDir != "R1" {
			return
		}
		_ = epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
			r.Participants = append(r.Participants, EpochParticipant{Kind: "raw-run", NativeHandle: "R2", RegisteredAt: "x"})
			return nil
		})
	}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (the racing P2 must be caught)", res.Disposition)
	}
	if !hasFinding(res.Findings, "unaccounted-participant:R2") {
		t.Fatalf("findings = %v, want unaccounted-participant:R2", res.Findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling", st)
	}
}

// TestCancelRepeatResumesCleanup: a first cancel fences and leaves the run pending
// on an unproven stop; a repeat against the cancelling epoch resumes cleanup (no
// re-fence, no authority restore) and completes to cancelled when the stop proves.
func TestCancelRepeatResumesCleanup(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: false}}

	first := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if first.Disposition != CancelDispositionPending {
		t.Fatalf("first disposition = %q, want cancellation-pending", first.Disposition)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelling {
		t.Fatalf("after first cancel epoch state = %q, want cancelling", st)
	}

	// The teardown now proves; a repeat resumes cleanup on the cancelling epoch.
	stopper.proven[fx.runDir] = true
	second := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if second.Disposition != CancelDispositionCancelled {
		t.Fatalf("second disposition = %q, want cancelled (findings=%v)", second.Disposition, second.Findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelled {
		t.Fatalf("after repeat epoch state = %q, want cancelled", st)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "released" {
		t.Fatalf("slot state = %q, want released", st)
	}
}

// TestCancelPendingOnUncompletedMutation: an admitted-not-completed mutation keeps
// the cancellation pending even when every process teardown proves — no premature
// cancelled.
func TestCancelPendingOnUncompletedMutation(t *testing.T) {
	fx := newCancelFixture(t, true)
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = []AdmittedMutation{{OpKey: "pr.publish", Status: "admitted"}}
		return nil
	}); err != nil {
		t.Fatalf("epochCAS seed mutation: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (uncompleted mutation)", res.Disposition)
	}
	if !hasFinding(res.Findings, "mutation-pending:pr.publish") {
		t.Fatalf("findings = %v, want mutation-pending:pr.publish", res.Findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling", st)
	}
}

// TestCancelNativeAdapterAbsentIsFindingNotSilence: with no native adapter wired, a
// native participant yields an explicit finding while the process teardown still
// accounts the run to cancelled.
func TestCancelNativeAdapterAbsentIsFindingNotSilence(t *testing.T) {
	fx := newCancelFixture(t, true)
	if err := RegisterEpochParticipant(fx.repo, fx.key, fx.epochID, EpochParticipant{Kind: "coordinator", NativeHandle: "turn-1"}); err != nil {
		t.Fatalf("RegisterEpochParticipant: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, native: nil, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled", res.Disposition)
	}
	if !hasFinding(res.Findings, "native-cancel-unavailable") {
		t.Fatalf("an absent adapter must produce a finding, not silence: findings=%v", res.Findings)
	}
}

// TestCancelNeverChargesOrResets: cancellation touches neither the change-owned
// suite budget nor the gate retry markers, and resets no gate-record retry state.
func TestCancelNeverChargesOrResets(t *testing.T) {
	fx := newCancelFixture(t, true)

	// Seed a consumed retry marker and a reserved suite attempt.
	if ok, err := ConsumeGateRetry(fx.repo, fx.key, 1, 2); err != nil || !ok {
		t.Fatalf("ConsumeGateRetry = (%v, %v), want (true, nil)", ok, err)
	}
	retryBefore, err := GateRetryUsage(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("GateRetryUsage: %v", err)
	}
	budgetKey := gatedrive.SuiteBudgetKey{RepoIdentity: fx.common, ChangeID: "42", Phase: "build"}
	if _, _, err := fx.store.ReserveSuiteAttempt(budgetKey, 3); err != nil {
		t.Fatalf("ReserveSuiteAttempt: %v", err)
	}
	usedBefore, limitBefore, err := fx.store.SuiteBudgetUsage(budgetKey)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage: %v", err)
	}

	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled", res.Disposition)
	}

	retryAfter, err := GateRetryUsage(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("GateRetryUsage after: %v", err)
	}
	if retryAfter != retryBefore {
		t.Fatalf("retry markers changed: before=%d after=%d — cancellation must not touch retry state", retryBefore, retryAfter)
	}
	usedAfter, limitAfter, err := fx.store.SuiteBudgetUsage(budgetKey)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage after: %v", err)
	}
	if usedAfter != usedBefore || limitAfter != limitBefore {
		t.Fatalf("suite budget changed: before=(%d,%d) after=(%d,%d) — cancellation charges no attempt", usedBefore, limitBefore, usedAfter, limitAfter)
	}
	rec, err := LoadGateRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadGateRecord after: %v", err)
	}
	if rec.Retry != RetryConsumed {
		t.Fatalf("gate record Retry = %q, want consumed (unchanged)", rec.Retry)
	}
	if rec.AttemptLimit != 2 {
		t.Fatalf("gate record AttemptLimit = %d, want 2 (unchanged)", rec.AttemptLimit)
	}
}

// TestCancelPendingWhileLaunchObligationUnresolved: even with every process teardown
// proven, an epoch-linked launch obligation the reconciler reports unsettled keeps
// the cancellation pending (a completed replacement must never first appear after a
// completed cancellation), surfacing the reconciler's findings.
func TestCancelPendingWhileLaunchObligationUnresolved(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	recon := &fakeLaunchReconciler{report: gatedrive.EpochLaunchReport{
		Accounted: false,
		Findings:  []string{"launch-pending:d1"},
	}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: recon}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (an unsettled launch obligation)", res.Disposition)
	}
	if !hasFinding(res.Findings, "launch-pending:d1") {
		t.Fatalf("findings = %v, want the reconciler's launch-pending:d1 surfaced", res.Findings)
	}
	if len(recon.calls) != 1 || recon.calls[0] != fx.worktree+"|"+fx.epochID {
		t.Fatalf("reconciler calls = %v, want [%s]", recon.calls, fx.worktree+"|"+fx.epochID)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling (durable fence held)", st)
	}
}

// TestCancelCompletesWhenLaunchObligationsSettle: with the reconciler reporting every
// launch obligation accounted and the rest of the accounting green, cancellation
// completes to cancelled.
func TestCancelCompletesWhenLaunchObligationsSettle(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	recon := okLaunchReconciler()
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: recon}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if len(recon.calls) != 1 || recon.calls[0] != fx.worktree+"|"+fx.epochID {
		t.Fatalf("reconciler calls = %v, want [%s]", recon.calls, fx.worktree+"|"+fx.epochID)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelled {
		t.Fatalf("epoch state = %q, want cancelled", st)
	}
}

// TestCancelReconcilerUnavailableFailsClosed: a nil launch reconciler is not silence
// — it is a finding and a fail-closed pending, mirroring the nil-stopper rule.
func TestCancelReconcilerUnavailableFailsClosed(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: nil}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (nil reconciler fails closed)", res.Disposition)
	}
	if !hasFinding(res.Findings, "launch-reconciler-unavailable") {
		t.Fatalf("findings = %v, want launch-reconciler-unavailable", res.Findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling", st)
	}
}

// TestRunCancelPublicEntry: the public RunCancel composes production seams and, over
// a run with no worktree slot and no native adapter, refuses cleanly when authority
// is wrong (here a wrong epoch) — proving the public signature is wired.
func TestRunCancelPublicEntry(t *testing.T) {
	fx := newCancelFixture(t, false)
	res := RunCancel(context.Background(), PlanningDeps{}, WorkspaceDeps{}, fx.repo, fx.key, "wrong-epoch", "human stop")
	if res.Disposition != CancelDispositionRefused {
		t.Fatalf("disposition = %q, want refused", res.Disposition)
	}
	if res.Operation != OperationRunCancel {
		t.Fatalf("operation = %q, want %q", res.Operation, OperationRunCancel)
	}
}
