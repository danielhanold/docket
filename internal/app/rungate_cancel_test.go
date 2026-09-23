package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
		// The slot records a real owning RunEpochID so the ownership-checked
		// teardown treats it as slotOwned (change 0435) — the same teardown behavior
		// the raw (epoch-less) reservation used to get, now anchored on true epoch
		// ownership rather than the worktree location alone.
		runDir := filepath.Join(worktree, "run-1")
		token, terr := fx.store.ReserveWorktreeExecutionForEpoch(common, worktree, ep.EpochID, nil)
		if terr != nil {
			t.Fatalf("ReserveWorktreeExecutionForEpoch: %v", terr)
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

// loadSlotEpoch reads the worktree slot's current RunEpochID.
func loadSlotEpoch(t *testing.T, store *gatedrive.Store, worktree string) string {
	t.Helper()
	slot, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	return slot.RunEpochID
}

// removeAdmissionRecord deletes the worktree slot's record file so the next slot
// write fails typed (ErrNotFound) — a deterministic durable-write failure. The
// path shape is the documented storage layout in admission.go's file header:
// <git-common-dir>/docket/gate-admission/v1/<admission-key>/record.json, where the
// admission key is the sha256 (lowercase hex) of the canonical, symlink-resolved
// worktree root (admissionKey). It fails loudly if the record is not where the
// layout says, rather than skipping — a moved constant must surface here.
func removeAdmissionRecord(t *testing.T, common, worktree string) {
	t.Helper()
	canon, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	sum := sha256.Sum256([]byte(canon))
	rec := filepath.Join(common, "docket", "gate-admission", "v1", hex.EncodeToString(sum[:]), "record.json")
	if _, err := os.Stat(rec); err != nil {
		t.Fatalf("admission record not at documented layout %q: %v", rec, err)
	}
	if err := os.Remove(rec); err != nil {
		t.Fatalf("remove admission record: %v", err)
	}
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
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want cleared (completed cancellation retires the owned released slot)", epo)
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
	// The terminal path now runs the bounded historical repair, which re-proves
	// quiescence through the launch reconciler; a nil reconciler is unverifiable and
	// refused by design, so an authorized terminal repeat injects okLaunchReconciler().
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")

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
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want cleared (completed cancellation retires the owned released slot)", epo)
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

// TestCancelNeverTouchesForeignSlot (AC4): a slot the worktree carries for a
// DIFFERENT epoch is never marked, stopped, or released by this epoch's cancel — a
// different nonempty RunEpochID is a foreign owner, surfaced informationally.
func TestCancelNeverTouchesForeignSlot(t *testing.T) {
	fx := newCancelFixture(t, false)
	// Occupy the worktree with a FOREIGN epoch's executing slot.
	ftoken, err := fx.store.ReserveWorktreeExecutionForEpoch(fx.common, fx.worktree, "foreign-epoch", nil)
	if err != nil {
		t.Fatalf("reserve foreign: %v", err)
	}
	if err := fx.store.ConfirmWorktreeExecution(fx.worktree, ftoken, "run-F", filepath.Join(fx.worktree, "run-F")); err != nil {
		t.Fatalf("confirm foreign: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (a foreign slot is not this epoch's obligation; findings=%v)", res.Disposition, res.Findings)
	}
	if len(stopper.calls) != 0 {
		t.Fatalf("a foreign slot's process must never be stopped: calls=%v", stopper.calls)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "executing" {
		t.Fatalf("foreign slot state = %q, want executing (untouched)", st)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "foreign-epoch" {
		t.Fatalf("foreign slot epoch = %q, want foreign-epoch (untouched)", epo)
	}
	if !hasFinding(res.Findings, "slot-foreign-owner") {
		t.Fatalf("findings = %v, want the informational slot-foreign-owner", res.Findings)
	}
}

// TestCancelLeavesUnlinkedEpochlessSlot (AC4): an epoch-less slot whose execution is
// NOT independently linked to this epoch's registered participants is left
// untouched, with an unresolved-ownership finding; cancellation still completes.
func TestCancelLeavesUnlinkedEpochlessSlot(t *testing.T) {
	fx := newCancelFixture(t, false)
	rtoken, err := fx.store.ReserveRawWorktreeExecution(fx.common, fx.worktree, nil)
	if err != nil {
		t.Fatalf("reserve raw: %v", err)
	}
	if err := fx.store.ConfirmWorktreeExecution(fx.worktree, rtoken, "run-X", filepath.Join(fx.worktree, "run-X")); err != nil {
		t.Fatalf("confirm raw: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if len(stopper.calls) != 0 {
		t.Fatalf("an unlinked epoch-less slot must not be stopped: calls=%v", stopper.calls)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "executing" {
		t.Fatalf("epoch-less slot state = %q, want executing (untouched)", st)
	}
	if !hasFinding(res.Findings, "slot-ownership-unresolved") {
		t.Fatalf("findings = %v, want slot-ownership-unresolved", res.Findings)
	}
}

// TestCancelStopsLinkedEpochlessSlot (AC4): an epoch-less slot IS torn down when its
// exact execution (RawRunDir) is independently linked to a registered execution
// participant of this epoch.
func TestCancelStopsLinkedEpochlessSlot(t *testing.T) {
	fx := newCancelFixture(t, false)
	runDir := filepath.Join(fx.worktree, "run-L")
	rtoken, err := fx.store.ReserveRawWorktreeExecution(fx.common, fx.worktree, nil)
	if err != nil {
		t.Fatalf("reserve raw: %v", err)
	}
	if err := fx.store.ConfirmWorktreeExecution(fx.worktree, rtoken, "run-L", runDir); err != nil {
		t.Fatalf("confirm raw: %v", err)
	}
	if err := RegisterEpochParticipant(fx.repo, fx.key, fx.epochID, EpochParticipant{Kind: "raw-run", NativeHandle: runDir}); err != nil {
		t.Fatalf("RegisterEpochParticipant: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "released" {
		t.Fatalf("linked epoch-less slot state = %q, want released", st)
	}
}

// TestCancelReleaseWriteFailureFailsClosed (AC5): a release whose durable write
// fails (the record vanishes between the proven stop and the release) keeps the
// cancellation pending — a successful process stop never proves the release was
// recorded.
func TestCancelReleaseWriteFailureFailsClosed(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	stopper.onStop = func(runDir string) {
		if runDir != fx.runDir {
			return
		}
		// Remove the slot record so the ReleaseWorktreeExecution CAS fails typed.
		removeAdmissionRecord(t, fx.common, fx.worktree)
	}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (release write failed; findings=%v)", res.Disposition, res.Findings)
	}
	if !hasFinding(res.Findings, "slot-release-failed") {
		t.Fatalf("findings = %v, want slot-release-failed", res.Findings)
	}
}

// TestCancelRetiresOwnedReleasedSlot (AC1/AC2 app half): completed cancellation
// releases AND detaches the slot — RunEpochID cleared, historical fields preserved.
func TestCancelRetiresOwnedReleasedSlot(t *testing.T) {
	fx := newCancelFixture(t, true)
	before, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	after, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.RunEpochID != "" {
		t.Fatalf("slot RunEpochID = %q, want cleared", after.RunEpochID)
	}
	if string(after.State) != "released" {
		t.Fatalf("slot state = %q, want released", after.State)
	}
	if after.RawRunID != before.RawRunID || after.RawRunDir != before.RawRunDir ||
		after.ExecutionGen != before.ExecutionGen || after.DriveID != before.DriveID ||
		after.Kind != before.Kind {
		t.Fatalf("retirement must preserve history: before=%+v after=%+v", before, after)
	}
}

// TestCancelPendingWhenRetirementFails (AC5): a retirement write failure keeps the
// epoch cancelling and the disposition pending — never a false cancelled.
func TestCancelPendingWhenRetirementFails(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler(),
		retire: func(worktree, epoch, token string) error { return fmt.Errorf("injected retire fault") }}
	res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (findings=%v)", res.Disposition, res.Findings)
	}
	if !hasFinding(res.Findings, "slot-retire-failed") {
		t.Fatalf("findings = %v, want slot-retire-failed", res.Findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling (fence held, ownership intact)", st)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != fx.epochID {
		t.Fatalf("slot epoch = %q, want retained %q", epo, fx.epochID)
	}
}

// TestCancelInterruptedBetweenRetireAndFinalizeConverges (AC5): retirement landed
// but cancelled was never persisted (simulated crash between the two writes); the
// retry revalidates, accepts the already-detached slot, and finishes the epoch
// transition — without touching a successor.
func TestCancelInterruptedBetweenRetireAndFinalizeConverges(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	// Reconstruct the crash state directly: the epoch record CAS has no seam, so
	// fence + full teardown + real retirement are driven here, leaving the epoch
	// cancelling with the slot already detached (exactly the interrupted state).
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error { r.State = EpochCancelling; return nil }); err != nil {
		t.Fatalf("fence: %v", err)
	}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}
	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ok, f, terr := reconcileEpochTeardown(seams, fx.repo, fx.key, ep); terr != nil || !ok {
		t.Fatalf("teardown = (%v,%v,%v), want accounted", ok, f, terr)
	}
	if ok, f := retireWorktreeSlotOwnership(seams, ep); !ok {
		t.Fatalf("retire = (false,%q), want retired", f)
	}
	// Crash happened here: slot detached, epoch still cancelling. The retry:
	res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("retry disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelled {
		t.Fatalf("epoch state = %q, want cancelled", st)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want still cleared", epo)
	}
}

// TestCancelRetireRaceWithSuccessorLeavesSuccessor (AC4): the slot is replaced by a
// successor between the cancel's load and its retire CAS — the retire refuses on
// the changed reservation, the re-read classifies the successor as foreign, and
// cancellation completes WITHOUT touching it (never retried with the successor's
// token).
func TestCancelRetireRaceWithSuccessorLeavesSuccessor(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	var raced bool
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}
	seams.retire = func(worktree, epoch, token string) error {
		if !raced {
			raced = true
			// The successor replaces the slot NOW: retire the old epoch out-of-band and
			// admit a new epoch's reservation (what a real winner would have produced).
			if err := fx.store.RetireWorktreeExecutionEpoch(worktree, epoch, token); err != nil {
				t.Fatalf("out-of-band retire: %v", err)
			}
			if _, err := fx.store.ReserveWorktreeExecutionForEpoch(fx.common, worktree, "successor-epoch", nil); err != nil {
				t.Fatalf("successor reserve: %v", err)
			}
		}
		return fx.store.RetireWorktreeExecutionEpoch(worktree, epoch, token) // now refuses ErrStaleRunEpoch
	}
	res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (successor is not our obligation; findings=%v)", res.Disposition, res.Findings)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "successor-epoch" {
		t.Fatalf("slot epoch = %q, want the untouched successor-epoch", epo)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "reserved" {
		t.Fatalf("successor slot state = %q, want reserved (untouched)", st)
	}
}

// TestCancelConcurrentReplayIsIdempotent (AC4): two sequential replays of a
// completed cancellation are no-ops (already-cancelled) leaving slot and epoch
// byte-stable.
func TestCancelConcurrentReplayIsIdempotent(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}
	if res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop"); res.Disposition != CancelDispositionCancelled {
		t.Fatalf("first = %q, want cancelled", res.Disposition)
	}
	for i := 0; i < 2; i++ {
		res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
		if res.Disposition != CancelDispositionAlreadyCancelled {
			t.Fatalf("replay %d = %q, want already-cancelled (findings=%v)", i, res.Disposition, res.Findings)
		}
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want cleared and stable", epo)
	}
}

// TestTerminalRepairRetiresHistoricalStaleSlot (AC6): a durably CANCELLED epoch
// whose released slot still carries its RunEpochID (the recorded incident shape:
// a pre-0435 cancel released but never retired) is repaired by an authorized repeat
// cancel — disposition cancelled/applied, slot detached, epoch state
// untouched-terminal — and a second repair is an idempotent no-op.
func TestTerminalRepairRetiresHistoricalStaleSlot(t *testing.T) {
	fx := newCancelFixture(t, true)
	// Manufacture the historical defect: release WITHOUT retirement, then force the
	// epoch terminal (what the pre-0435 cancel produced).
	slot, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := fx.store.ReleaseWorktreeExecution(fx.worktree, slot.ReservationToken); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error { r.State = EpochCancelled; return nil }); err != nil {
		t.Fatalf("force cancelled: %v", err)
	}
	res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human repair")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (historical retirement applied; findings=%v)", res.Disposition, res.Findings)
	}
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want cleared", epo)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelled {
		t.Fatalf("epoch state = %q, want cancelled (never regressed)", st)
	}
	// Repeated repair is a no-op.
	res2 := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human repair")
	if res2.Disposition != CancelDispositionAlreadyCancelled || res2.Result != ResultNoOp {
		t.Fatalf("repeat = (%q,%q), want (already-cancelled,no-op)", res2.Disposition, res2.Result)
	}
}

// TestTerminalRepairSupersededSlot (AC6): the same repair works for a SUPERSEDED
// epoch's stale released slot, and never regresses the superseded state.
func TestTerminalRepairSupersededSlot(t *testing.T) {
	fx := newCancelFixture(t, true)
	slot, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := fx.store.ReleaseWorktreeExecution(fx.worktree, slot.ReservationToken); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error { r.State = EpochSuperseded; return nil }); err != nil {
		t.Fatalf("force superseded: %v", err)
	}
	res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human repair")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want cleared", epo)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochSuperseded {
		t.Fatalf("epoch state = %q, want superseded (never regressed)", st)
	}
}

// TestTerminalRepairRefusesUnsafeHistories (AC6): each unsafe terminal history is
// refused with a specific finding, with NO slot or epoch mutation, and never
// cancellation-pending over durable terminal state.
func TestTerminalRepairRefusesUnsafeHistories(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(t *testing.T, fx cancelFixture) cancelSeams
		finding string
	}{
		{"busy-claim-unresolved-relaunch", func(t *testing.T, fx cancelFixture) cancelSeams {
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{},
				launches: &fakeLaunchReconciler{report: gatedrive.EpochLaunchReport{Accounted: false, Findings: []string{"claim-busy:d1"}}}}
		}, "claim-busy:d1"},
		{"contradictory-mutation", func(t *testing.T, fx cancelFixture) cancelSeams {
			if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
				r.AdmittedMutations = []AdmittedMutation{{OpKey: "pr.publish", Status: "admitted"}}
				return nil
			}); err != nil {
				t.Fatalf("seed mutation: %v", err)
			}
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}
		}, "mutation-pending:pr.publish"},
		{"unreadable-launch-evidence", func(t *testing.T, fx cancelFixture) cancelSeams {
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{},
				launches: &fakeLaunchReconciler{err: fmt.Errorf("injected")}}
		}, "launch-reconcile-failed"},
		{"nonreleased-owned-slot", func(t *testing.T, fx cancelFixture) cancelSeams {
			// slot left executing (fixture default) — owned but not released.
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}
		}, "slot-not-released"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCancelFixture(t, true)
			if tc.name != "nonreleased-owned-slot" {
				slot, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
				if err != nil {
					t.Fatalf("load: %v", err)
				}
				if err := fx.store.ReleaseWorktreeExecution(fx.worktree, slot.ReservationToken); err != nil {
					t.Fatalf("release: %v", err)
				}
			}
			seams := tc.arrange(t, fx)
			if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error { r.State = EpochCancelled; return nil }); err != nil {
				t.Fatalf("force cancelled: %v", err)
			}
			epochBefore := loadSlotEpoch(t, fx.store, fx.worktree)
			res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human repair")
			if res.Disposition != CancelDispositionRefused {
				t.Fatalf("disposition = %q, want refused (findings=%v)", res.Disposition, res.Findings)
			}
			if !hasFinding(res.Findings, tc.finding) {
				t.Fatalf("findings = %v, want %q", res.Findings, tc.finding)
			}
			if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelled {
				t.Fatalf("epoch state = %q, want cancelled (no regression, no revival)", st)
			}
			if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != epochBefore {
				t.Fatalf("slot epoch changed %q->%q under a refused repair", epochBefore, epo)
			}
		})
	}
}

// TestGuardianReapsButNeverRetires: the death guardian's fence+reap releases the
// proven-stopped slot but RETAINS RunEpochID and leaves the epoch CANCELLING —
// only authorized run.cancel completion retires (spec "The death guardian may
// perform teardown but never retires epoch ownership"). The contract is proven
// against the shared reconcileEpochTeardown, which the guardian composes and which
// performs no retirement; retirement stays exclusively in runCancel's post-accounted
// completion block and repairTerminalEpoch, neither of which the guardian reaches.
func TestGuardianReapsButNeverRetires(t *testing.T) {
	fx := newCancelFixture(t, true)
	// guardianFenceAndReap composes productionCancelSeams, whose stopper/reconciler
	// reach the real process service — unavailable here. Drive its exact sequence
	// with injected seams instead: fence, then the SAME teardown accounting, and
	// assert what the guardian contract asserts — no retirement, no finalize.
	guardianFenceAndReapWithSeams := func() {
		ferr := epochCAS(fx.repo, fx.key, func(rec *EpochRecord) error {
			if rec.State == EpochActive {
				rec.State = EpochCancelling
			}
			return nil
		})
		if ferr != nil {
			t.Fatalf("fence: %v", ferr)
		}
		ep, _, err := LoadEpochRecord(fx.repo, fx.key)
		if err != nil {
			t.Fatalf("LoadEpochRecord: %v", err)
		}
		_, _, _ = reconcileEpochTeardown(cancelSeams{store: fx.store,
			stopper:  &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}},
			launches: okLaunchReconciler()}, fx.repo, fx.key, ep)
	}
	guardianFenceAndReapWithSeams()
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling (guardian never finalizes)", st)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "released" {
		t.Fatalf("slot state = %q, want released (guardian reaps)", st)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != fx.epochID {
		t.Fatalf("slot epoch = %q, want retained %q (guardian never retires ownership)", epo, fx.epochID)
	}
	// The authorized completion then retires and finalizes.
	res := runCancel(cancelSeams{store: fx.store,
		stopper:  &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}},
		launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("authorized completion = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want cleared by the authorized path", epo)
	}
}

// TestFinalizeGateAdmitsAfterRetirement (AC1): before retirement the epoch-owned
// released slot blocks an epoch-less raw/finalize launch (rawStaleEpochRefusal's
// stale-run-epoch) and a different-epoch reservation (reserveWorktreeExecution's
// between-drives fence); after authorized cancellation retires the ownership, both
// admit again — the released slot is genuinely reusable.
func TestFinalizeGateAdmitsAfterRetirement(t *testing.T) {
	fx := newCancelFixture(t, true)
	slot, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := fx.store.ReleaseWorktreeExecution(fx.worktree, slot.ReservationToken); err != nil {
		t.Fatalf("release: %v", err)
	}
	// BEFORE: the released slot still owns the worktree. The raw pre-check defers a
	// RELEASED slot to the reserve (change 0446), which is the authority that refuses
	// while the owning epoch is live — even with the production settlement read wired.
	fx.store.SetEpochSettledResolver(epochSettledResolver(fx.common))
	if _, err := fx.store.ReserveRawWorktreeExecution(fx.common, fx.worktree, nil); !isGateOwnership(err, gatedrive.ErrStaleRunEpoch) {
		t.Fatalf("pre-retirement: an epoch-less raw reserve must be refused stale-run-epoch, got %v", err)
	}
	if _, err := fx.store.ReserveWorktreeExecutionForEpoch(fx.common, fx.worktree, "replacement-epoch", nil); err == nil {
		t.Fatal("pre-retirement: a different epoch's reservation must be refused")
	}
	// Authorized cancellation retires (slot already released; teardown is vacuous).
	res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancel = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	// AFTER: the epoch-less finalize gate no longer refuses…
	if _, refused := rawStaleEpochRefusal(fx.store, fx.worktree); refused {
		t.Fatal("post-retirement: rawStaleEpochRefusal must not refuse an epoch-less launch")
	}
	// …and a replacement epoch's build-gate reservation admits.
	if _, err := fx.store.ReserveWorktreeExecutionForEpoch(fx.common, fx.worktree, "replacement-epoch", nil); err != nil {
		t.Fatalf("post-retirement replacement reserve: %v", err)
	}
}

// TestRetirementDoesNotUnfenceOldEpochLaunches (AC8): after retirement the OLD
// epoch's fresh start is still refused by the 437 launch gate (epochLaunchGate) —
// clearing slot ownership never revives the cancelled epoch's launch authority, and
// the refused gate never runs the reservation body.
func TestRetirementDoesNotUnfenceOldEpochLaunches(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	if res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop"); res.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancel = %q, want cancelled", res.Disposition)
	}
	gate := epochLaunchGate(fx.common)
	reserveRan := false
	err := gate(fx.epochID, fx.worktree, func() error { reserveRan = true; return nil })
	if err == nil {
		t.Fatal("the cancelled epoch's launch authorization must be refused after retirement")
	}
	if reserveRan {
		t.Fatal("the refused gate must never run the reservation body")
	}
}

// TestRepairChargesNothing (AC8): terminal repair — like cancellation — touches
// neither the suite budget nor the gate retry markers. This mirrors
// TestCancelNeverChargesOrResets (same seeding and asserts) with the historical
// stale-slot repair arrangement of TestTerminalRepairRetiresHistoricalStaleSlot
// (release WITHOUT retirement + epoch forced cancelled) placed between the seeding
// and the accounting-neutrality asserts.
func TestRepairChargesNothing(t *testing.T) {
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

	// Manufacture the historical defect the repair path addresses: release WITHOUT
	// retirement, then force the epoch terminal (what the pre-0435 cancel produced).
	slot, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := fx.store.ReleaseWorktreeExecution(fx.worktree, slot.ReservationToken); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error { r.State = EpochCancelled; return nil }); err != nil {
		t.Fatalf("force cancelled: %v", err)
	}

	res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human repair")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (repair applied; findings=%v)", res.Disposition, res.Findings)
	}

	retryAfter, err := GateRetryUsage(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("GateRetryUsage after: %v", err)
	}
	if retryAfter != retryBefore {
		t.Fatalf("retry markers changed: before=%d after=%d — repair must not touch retry state", retryBefore, retryAfter)
	}
	usedAfter, limitAfter, err := fx.store.SuiteBudgetUsage(budgetKey)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage after: %v", err)
	}
	if usedAfter != usedBefore || limitAfter != limitBefore {
		t.Fatalf("suite budget changed: before=(%d,%d) after=(%d,%d) — repair charges no attempt", usedBefore, limitBefore, usedAfter, limitAfter)
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

// TestRunCancelWinsFromCompletingEpoch: an explicit human cancellation WINS from a
// completing (successful, mid-closeout) epoch — the fence flips completing→cancelling
// and the ordinary teardown/accounting runs to a proven cancellation, never a
// completing/completed relabelling. Completion then loses (change 0441): its
// completing→completed CAS refuses once this fence lands.
func TestRunCancelWinsFromCompletingEpoch(t *testing.T) {
	fx := newCancelFixture(t, true)
	forceEpochState(t, fx.repo, fx.key, EpochCompleting)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	st := loadEpochState(t, fx.repo, fx.key)
	if st != EpochCancelled && st != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling or cancelled — never completing/completed", st)
	}
}

// TestRunCancelRefusesCompletedEpoch: a completed (successfully closed-out) run
// cannot be cancelled — a no-op refusal carrying the completed-run explanation, never
// a state regression and never cancellation-pending over durable terminal state
// (change 0441).
func TestRunCancelRefusesCompletedEpoch(t *testing.T) {
	fx := newCancelFixture(t, true)
	forceEpochState(t, fx.repo, fx.key, EpochCompleted)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")

	if res.Disposition != CancelDispositionRefused {
		t.Fatalf("disposition = %q, want refused (findings=%v)", res.Disposition, res.Findings)
	}
	if !hasFinding(res.Findings, "run-completed") {
		t.Fatalf("findings = %v, want a run-completed finding", res.Findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCompleted {
		t.Fatalf("epoch state = %q, want completed unchanged (no regression)", st)
	}
	if len(stopper.calls) != 0 {
		t.Fatalf("a refused cancel of a completed run must stop nothing, got %v", stopper.calls)
	}
}

// isGateOwnership reports whether err is a gatedrive ownership error of kind.
func isGateOwnership(err error, kind gatedrive.OwnershipErrorKind) bool {
	oe, ok := gatedrive.AsOwnershipError(err)
	return ok && oe.Kind == kind
}

// TestRawLaunchSettlesSettledEpochReleasedSlot (change 0446 spec §§2, 5): a
// released slot whose leftover RunEpochID names a COMPLETED or confirmed-CANCELLED
// epoch no longer blocks an epoch-less raw/finalize launch — the raw pre-check
// defers the released slot to the reserve, which settles the epoch through the
// production settlement read and exact-token retirement, so a successfully
// completed run is never asked to be cancelled. An active, cancelling, or
// completing epoch still owns its worktree: the reserve refuses stale-run-epoch and
// the slot is left untouched.
func TestRawLaunchSettlesSettledEpochReleasedSlot(t *testing.T) {
	cases := []struct {
		state   epochState
		settled bool
	}{
		{EpochCompleted, true},
		{EpochCancelled, true},
		{EpochActive, false},
		{EpochCancelling, false},
		{EpochCompleting, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			fx := newCancelFixture(t, true)
			slot, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if err := fx.store.ReleaseWorktreeExecution(fx.worktree, slot.ReservationToken); err != nil {
				t.Fatalf("release: %v", err)
			}
			if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
				r.State = tc.state
				return nil
			}); err != nil {
				t.Fatalf("set epoch state: %v", err)
			}
			fx.store.SetEpochSettledResolver(epochSettledResolver(fx.common))

			if _, refused := rawStaleEpochRefusal(fx.store, fx.worktree); refused {
				t.Fatal("a released slot must defer to the reserve, not be refused by the raw pre-check")
			}
			_, err = fx.store.ReserveRawWorktreeExecution(fx.common, fx.worktree, nil)
			if tc.settled {
				if err != nil {
					t.Fatalf("raw reserve over a %s epoch's released slot: %v", tc.state, err)
				}
				if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
					t.Fatalf("slot epoch = %q, want retired", epo)
				}
				return
			}
			if !isGateOwnership(err, gatedrive.ErrStaleRunEpoch) {
				t.Fatalf("raw reserve over a %s epoch's released slot = %v, want stale-run-epoch", tc.state, err)
			}
			if st, epo := loadSlotState(t, fx.store, fx.worktree), loadSlotEpoch(t, fx.store, fx.worktree); st != "released" || epo != fx.epochID {
				t.Fatalf("refused slot changed: state %q epoch %q", st, epo)
			}
		})
	}
}

// TestRawStaleEpochRefusalStillFencesBusySlot: the raw pre-check keeps refusing a
// BUSY slot another epoch owns — only a released slot defers to the reserve.
func TestRawStaleEpochRefusalStillFencesBusySlot(t *testing.T) {
	fx := newCancelFixture(t, true)
	fx.store.SetEpochSettledResolver(epochSettledResolver(fx.common))
	if _, refused := rawStaleEpochRefusal(fx.store, fx.worktree); !refused {
		t.Fatal("an executing epoch-owned slot must be refused stale-run-epoch by the raw pre-check")
	}
}

// TestRawAdmissionStoreWiresEpochSettledResolver: the raw launch path's admission
// store carries the production settlement read (change 0446) — without it a raw
// reserve over a completed run's released slot would refuse stale-run-epoch.
func TestRawAdmissionStoreWiresEpochSettledResolver(t *testing.T) {
	repo := newGateRepo(t)
	_, _, store, ok := resolveWorktreeAdmission(repo)
	if !ok {
		t.Fatal("resolveWorktreeAdmission: repo not resolved as a worktree")
	}
	if !store.EpochSettledResolverWired() {
		t.Fatal("raw admission store: epoch settlement resolver not wired")
	}
}
